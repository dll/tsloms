package model

import (
	"fmt"
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
	return db.AutoMigrate(
		&Device{},
		&PacketLog{},
		&FaultRecord{},
		&WorkOrder{},
		&User{},
		&Department{},
		&OperationLog{},
		&DeviceMedia{},
		&DeviceMaterial{},
		&Feedback{},
		&AIConfig{},
		&AIUsage{},
		&AIPrediction{},
	)
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
