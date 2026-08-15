package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tsloms/server/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 全局数据库句柄
var DB *gorm.DB

// RDB 全局 Redis 句柄
var RDB *redis.Client

// InitDB 初始化数据库连接，支持 MySQL/SQLite 双模式
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	if cfg.DBDriver == "sqlite" {
		// 开发/SQLite 模式：使用文件库 tsloms.db，无需 MySQL
		name := cfg.DBName
		if name == "" || name == "tsloms" || name == "tsloms.db" {
			name = "tsloms.db"
		}
		db, err = gorm.Open(sqlite.Open(name), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
	} else {
		// MySQL 模式：与 EQS 共享实例，独立数据库 tsloms
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
	}
	if err != nil {
		return nil, err
	}

	if err := AutoMigrate(db); err != nil {
		return nil, err
	}

	DB = db
	return db, nil
}

// AutoMigrate 自动迁移全部域模型
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&Device{},
		&PacketLog{},
		&FaultRecord{},
		&WorkOrder{},
		&User{},
		&Department{},
		&OperationLog{},
		&DeviceMedia{},
		&Feedback{},
		&AIConfig{},
		&AIUsage{},
		&AIPrediction{},
		&FirmwarePackage{},
		&FirmwareUpgradeRecord{},
		&Material{},
		&MaterialStock{},
		&Supplier{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&RepairExpense{},
		&AIReport{},
		&AIAdvice{},
		&Permission{},
		&Role{},
		&RolePermission{},
		&UserPermission{},
	); err != nil {
		return err
	}
	// 数据迁移：旧版故障状态 active → occurred（四态模型引入后）
	db.Model(&FaultRecord{}).Where("status = ?", "active").Update("status", FaultStatusOccurred)

	// 数据合并：旧的设备耗材台账(device_materials)并入统一物料档案(materials)
	// 保留设备维度 device_hw_id，写入初始库存流水，迁移完成后删除旧表
	MigrateLegacyDeviceMaterials(db)

	// 初始化 RBAC 权限字典与内置角色（幂等）
	SeedRBAC(db)
	return nil
}

// SeedRBAC 初始化权限字典与内置角色（admin/operator/viewer）及其默认权限，幂等
func SeedRBAC(db *gorm.DB) error {
	// 1) 权限字典：按 code 幂等插入
	permIDByCode := map[string]uint{}
	for _, p := range AllPermissions {
		var existing Permission
		err := db.Where("code = ?", p.Code).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if e := db.Create(&p).Error; e != nil {
				return e
			}
			permIDByCode[p.Code] = p.ID
		} else if err != nil {
			return err
		} else {
			permIDByCode[p.Code] = existing.ID
			// 同步名称/模块/排序，保持与新版本一致
			db.Model(&existing).Updates(map[string]interface{}{
				"name": p.Name, "module": p.Module, "sort": p.Sort,
			})
		}
	}

	// 2) 内置角色 + 默认权限
	builtins := []struct {
		code string
		name string
	}{
		{BuiltinRoleAdmin, "管理员"},
		{BuiltinRoleOperator, "运维人员"},
		{BuiltinRoleViewer, "查看人员"},
	}
	for _, b := range builtins {
		var role Role
		err := db.Where("code = ?", b.code).First(&role).Error
		if err == gorm.ErrRecordNotFound {
			role = Role{Code: b.code, Name: b.name, Builtin: true}
			if e := db.Create(&role).Error; e != nil {
				return e
			}
		} else if err != nil {
			return err
		}

		// 同步该角色默认权限（先删后插，保证与代码一致）
		db.Where("role_id = ?", role.ID).Delete(&RolePermission{})
		for _, code := range BuiltinRolePerms[b.code] {
			pid, ok := permIDByCode[code]
			if !ok {
				continue
			}
			if e := db.Create(&RolePermission{
				RoleID: role.ID, PermissionID: pid, RoleCode: role.Code,
			}).Error; e != nil {
				return e
			}
		}
	}
	return nil
}

// MigrateLegacyDeviceMaterials 将旧 device_materials 表数据合并进新的 materials 表
// 幂等：同名物料编码已存在则跳过仅绑定设备；迁移完成后删除旧表释放空间
func MigrateLegacyDeviceMaterials(db *gorm.DB) error {
	// 旧表不存在（全新库）则静默跳过
	if !db.Migrator().HasTable("device_materials") {
		return nil
	}
	var legacyCount int64
	if err := db.Table("device_materials").Count(&legacyCount).Error; err != nil {
		return err
	}
	if legacyCount == 0 {
		// 无数据：直接删除旧表
		return db.Migrator().DropTable("device_materials")
	}

	type legacyRow struct {
		ID         uint
		DeviceHwID uint32
		Name       string
		PartNo     string
		Spec       string
		Quantity   int
		Unit       string
		Threshold  int
		Note       string
		CreatedAt  time.Time
	}
	var rows []legacyRow
	if err := db.Table("device_materials").Find(&rows).Error; err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			code := r.PartNo
			if code == "" {
				code = r.Name
			}
			if code == "" {
				continue
			}
			// 同名物料已存在则跳过，仅补充设备绑定
			var exists Material
			err := tx.Where("code = ?", code).First(&exists).Error
			if err == nil {
				if exists.DeviceHwID == nil && r.DeviceHwID > 0 {
					hw := r.DeviceHwID
					if err := tx.Model(&exists).Update("device_hw_id", &hw).Error; err != nil {
						return err
					}
				}
				continue
			} else if err != gorm.ErrRecordNotFound {
				return err
			}

			cat := "其他"
			switch {
			case containsAny(r.Name, "灯", "LED", "泡"):
				cat = "灯泡"
			case containsAny(r.Name, "电源", "驱动"):
				cat = "电源"
			case containsAny(r.Name, "控制", "模块"):
				cat = "控制器"
			case containsAny(r.Name, "线", "缆"):
				cat = "线缆"
			}

			var hwPtr *uint32
			if r.DeviceHwID > 0 {
				hw := r.DeviceHwID
				hwPtr = &hw
			}
			m := Material{
				Code: code, Name: r.Name, Category: cat, Spec: r.Spec,
				Unit: r.Unit, Stock: r.Quantity, Threshold: r.Threshold,
				DeviceHwID: hwPtr, Note: r.Note, Status: "active",
				CreatedAt: r.CreatedAt, UpdatedAt: time.Now(),
			}
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
			if m.Stock > 0 {
				if err := tx.Create(&MaterialStock{
					MaterialID: m.ID, MaterialName: m.Name, Type: StockTypeIn,
					Quantity: m.Stock,
					RefType:  "adjust", Operator: "system", Note: "旧耗材台账合并初始库存",
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// 事务外删除旧表，避免事务内嵌套迁移导致连接死锁
	return db.Migrator().DropTable("device_materials")
}

// containsAny 判断 s 是否包含任意子串（大小写不敏感）
func containsAny(s string, subs ...string) bool {
	ls := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(ls, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// InitRedis 初始化 Redis 连接，使用 REDIS_DB 索引隔离（TSLOMS 使用 DB 1）
func InitRedis(cfg *config.Config) error {
	RDB = redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       cfg.RedisDB,
	})
	return nil
}

// SeedAdmin 初始化默认管理员账户（仅在 users 表为空时创建）
func SeedAdmin() error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	var count int64
	DB.Model(&User{}).Count(&count)
	if count > 0 {
		return nil
	}

	admin := User{
		Username:     "admin",
		PasswordHash: HashPassword("admin123"),
		Role:         RoleAdmin,
	}
	return DB.Create(&admin).Error
}

// InitTestDB 创建内存 SQLite 数据库（仅供单元测试）
func InitTestDB() *gorm.DB {
	// 使用独立名称的共享内存库，避免测试间数据串扰
	name := fmt.Sprintf("file:tsloms_test_%x?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic(err)
	}
	// 单连接串行使用，避免共享缓存并发问题
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := AutoMigrate(db); err != nil {
		panic(err)
	}
	DB = db
	return db
}
