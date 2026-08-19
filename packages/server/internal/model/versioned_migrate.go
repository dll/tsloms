package model

// versioned_migrate.go —— CD-P0-01 数据库版本化迁移。
//
// 目标：把「启动时无条件 AutoMigrate 隐式改库」重构为「显式版本化迁移」。
//   - 维护 schema_migrations 版本表，每个一次性/有副作用步骤严格按版本只执行一次；
//   - MySQL 生产用 GET_LOCK('tsloms_migrate', timeout) 保证单实例迁移；
//   - 含 DropTable / CREATE 唯一索引等 DDL 的版本，执行前强制备份，备份缺失/失败即 fail-closed；
//   - 纯幂等启动逻辑（结构基座、RBAC、区划种子）保持每启动重放安全，不进版本表；
//   - 不改变任何业务逻辑、字段、既有表/索引名。

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/tsloms/server/internal/config"
	"gorm.io/gorm"
)

// schemaMigrations 版本迁移表（基础设施元数据表）。
// 独立于 38 张业务表，不在 AutoMigrate 的模型清单里；由 MigrateDatabaseVersioned 自建。
type schemaMigrations struct {
	Version   string `gorm:"column:version;type:varchar(64);primaryKey"` // '0001_schema' 等
	Name      string `gorm:"column:name;type:varchar(255)"`              // 人类可读名称
	AppliedAt string `gorm:"column:applied_at;type:varchar(64)"`         // RFC3339 时间戳（跨方言安全）
	AppliedBy string `gorm:"column:applied_by;type:varchar(64)"`         // 实例标识，便于审计
}

// TableName 指定版本表表名（与业务模型解耦，避免误解为业务表）。
func (schemaMigrations) TableName() string { return "schema_migrations" }

// migrationStep 一次版本化迁移步骤的定义。
type migrationStep struct {
	Version     string                  // 有序版本号，如 '0001'，字典序即执行序
	Name        string                  // 描述
	NeedsBackup bool                    // 含 DropTable/DDL 需备份（fail-closed）
	Fn          func(db *gorm.DB) error // 迁移执行体；必须幂等/可重放安全
}

// orderedMigrations 全量有序迁移列表（新增步骤追加在末尾，禁止改动已发布版本对应 Fn）。
var orderedMigrations = []migrationStep{
	{
		Version: "0001",
		Name:    "gorm structure baseline + active->occurred",
		// 0001 结构基座：GORM AutoMigrate 全部既有表（幂等，补列加表天然可重放）
		// + active→occurred 状态升级（全量 UPDATE 幂等）。失败即 fail-closed。
		Fn: func(db *gorm.DB) error {
			if err := migrateStructureBaseline(db); err != nil {
				return err
			}
			return nil
		},
	},
	{
		Version:     "0002",
		Name:        "uk_wo_active_scope unique index (create)",
		NeedsBackup: true, // CREATE UNIQUE INDEX 属 DDL，有副作用
		Fn: func(db *gorm.DB) error {
			// migrateWorkOrderActiveUnique 内部已 HasIndex 幂等（清理重复活跃单 + 回填 scope + 建索引）。
			return migrateWorkOrderActiveUnique(db)
		},
	},
	{
		Version:     "0003",
		Name:        "merge legacy device_materials into materials (drop old table)",
		NeedsBackup: true, // DropTable 不可回滚，必须先备份
		Fn: func(db *gorm.DB) error {
			// 旧表存在则合并并删除；全新库无旧表则幂等跳过。严格一次：版本表记录后不再重放。
			return MigrateLegacyDeviceMaterials(db)
		},
	},
	{
		Version: "0004",
		Name:    "seed super admin (first create)",
		Fn: func(db *gorm.DB) error {
			// 超级管理员首次创建。账号已存在则幂等跳过。
			saPwd, err := SeedSuperAdmin(db, config.Get().SuperAdminPwd)
			if err != nil {
				return err
			}
			// 密码走安全渠道打印一次，不落普通审计/统计日志明文（审计 BLOCK-1）。
			if saPwd != "" {
				log.Printf("[TSLOMS][migration:0004] 超级管理员账号 %s 已创建，初始密码（仅显示一次，请立即保存并修改）: %s",
					SuperAdminUsername, saPwd)
			}
			return nil
		},
	},
}

// migrateStructureBaseline GORM 全量结构基座（0001 的独立实现，供版本化调用）。
func migrateStructureBaseline(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&Device{},
		&PacketLog{},
		&FaultRecord{},
		&FaultEvidence{},
		&FaultCase{},
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
		&Notification{},
		&NotificationRead{},
		&Warning{},
		&WarningRule{},
		&Area{},
		&Crossing{},
		&PatrolTask{},
		&PatrolRecord{},
		&ModuleToggle{},
		&LicenseState{},
	); err != nil {
		return err
	}
	// 数据迁移：旧版故障状态 active → occurred（四态模型引入后）。幂等：已 occurred 不再匹配。
	return db.Model(&FaultRecord{}).
		Where("status = ?", "active").
		Update("status", FaultStatusOccurred).Error
}

// MigrateDatabaseVersioned 执行显式版本化迁移。
//
// 语义（CD-P0-01）：
//  1. 自建 schema_migrations 版本表；
//  2. MySQL 生产用 GET_LOCK('tsloms_migrate', timeout) 单实例锁；SQLite 测试走简化无锁路径；
//  3. 对每个未应用版本：若该版本含 DDL/DropTable（NeedsBackup），执行前强制备份，失败 fail-closed；
//     SQLite 测试跳过备份；
//  4. 版本执行成功写入 schema_migrations；某一步失败立即 return error，不再继续后续版本；
//  5. 纯幂等启动逻辑（SeedRBAC / SeedAreas / active→occurred 兜底）在版本管道之后每启动重放安全执行。
func MigrateDatabaseVersioned(db *gorm.DB) error {
	if db == nil {
		return errors.New("MigrateDatabaseVersioned: 数据库未初始化")
	}
	if err := ensureSchemaMigrationsTable(db); err != nil {
		return fmt.Errorf("创建 schema_migrations 版本表失败: %w", err)
	}

	isMySQL := db.Dialector.Name() == "mysql"

	// ---------- 单实例锁（MySQL）----------
	lockTimeout := 300 // 秒；大表 CREATE UNIQUE INDEX 预留充分
	if isMySQL {
		got, err := acquireMigrateLock(db, lockTimeout)
		if err != nil {
			return fmt.Errorf("获取迁移锁失败(fail-closed): %w", err)
		}
		if !got {
			return fmt.Errorf("获取迁移锁超时(%ds)，可能已有其它实例在迁移；拒绝启动（fail-closed）", lockTimeout)
		}
		defer releaseMigrateLock(db)
	}

	// 已应用版本集合
	applied := map[string]bool{}
	if err := loadAppliedVersions(db, applied); err != nil {
		return err
	}

	instanceID := instanceIdentifier()

	for _, step := range orderedMigrations {
		if applied[step.Version] {
			continue // 已应用，跳过
		}

		// DDL/DropTable 版本：执行前强制备份（fail-closed）。SQLite 测试跳过备份。
		if step.NeedsBackup {
			if isMySQL {
				if err := backupDatabaseBeforeDDL(db); err != nil {
					return fmt.Errorf("迁移 %s(%s) 前备份失败，已阻断启动（fail-closed）: %w", step.Version, step.Name, err)
				}
			}
		}

		if err := step.Fn(db); err != nil {
			return fmt.Errorf("迁移 %s(%s) 失败，已终止后续迁移（fail-closed）: %w", step.Version, step.Name, err)
		}
		if err := recordAppliedVersion(db, step.Version, step.Name, instanceID); err != nil {
			return fmt.Errorf("记录迁移版本 %s 失败: %w", step.Version, err)
		}
		log.Printf("[TSLOMS][migration] 已应用版本 %s (%s)", step.Version, step.Name)
	}

	// ---------- 纯幂等启动逻辑（每启动重放安全，不进版本表）----------
	if err := SeedRBAC(db); err != nil {
		return fmt.Errorf("幂等种子 SeedRBAC 失败: %w", err)
	}
	SeedAreas(db)

	return nil
}

// acquireMigrateLock MySQL 单实例锁；SQLite 直接返回 true（简化路径）。
func acquireMigrateLock(db *gorm.DB, timeout int) (bool, error) {
	var ok int
	// GET_LOCK 返回 1=获取成功, 0=超时, NULL=错误；同一连接内需 RELEASE_LOCK。
	err := db.Raw("SELECT GET_LOCK(?, ?)", "tsloms_migrate", timeout).Scan(&ok).Error
	if err != nil {
		return false, err
	}
	return ok == 1, nil
}

// releaseMigrateLock 释放 MySQL 迁移锁（best-effort）。
func releaseMigrateLock(db *gorm.DB) {
	_ = db.Exec("SELECT RELEASE_LOCK(?)", "tsloms_migrate").Error
}

// ensureSchemaMigrationsTable 自建版本表（跨方言：MySQL DATETIME / SQLite 均用字符串时间戳）。
func ensureSchemaMigrationsTable(db *gorm.DB) error {
	if db.Migrator().HasTable(&schemaMigrations{}) {
		return nil
	}
	return db.AutoMigrate(&schemaMigrations{})
}

// loadAppliedVersions 读取已应用版本集合。
func loadAppliedVersions(db *gorm.DB, into map[string]bool) error {
	var rows []schemaMigrations
	if err := db.Find(&rows).Error; err != nil {
		return fmt.Errorf("读取已应用迁移版本失败: %w", err)
	}
	for _, r := range rows {
		into[r.Version] = true
	}
	return nil
}

// recordAppliedVersion 记录已应用版本。
func recordAppliedVersion(db *gorm.DB, version, name, by string) error {
	return db.Create(&schemaMigrations{
		Version:   version,
		Name:      name,
		AppliedAt: time.Now().UTC().Format(time.RFC3339),
		AppliedBy: by,
	}).Error
}

// instanceIdentifier 迁移者标识（便于审计哪台实例执行）。
func instanceIdentifier() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "unknown"
}

// mysqlBackupCfg 备份所需凭据/目标目录解析结果。
type mysqlBackupCfg struct {
	User, Password, Host, Port, DBName string
}

// resolveBackupCreds 从 config + /etc/tsloms/tsloms.env 解析备份凭据。
// 若 DB 密码为空（无法备份）→ 返回错误，触发 fail-closed。
func resolveBackupCreds() (*mysqlBackupCfg, error) {
	c := config.Get()
	user, password, host, port, dbname := c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName
	if dbname == "" {
		return nil, errors.New("DB_NAME 为空，无法定位备份数据库")
	}
	// 增强：/etc/tsloms/tsloms.env 中 DB_USER/DB_PASSWORD 优先覆盖（部署环境惯例）
	if envFile := "/etc/tsloms/tsloms.env"; fileExists(envFile) {
		if v, ok := envValue(envFile, "DB_USER"); ok && v != "" {
			user = v
		}
		if v, ok := envValue(envFile, "DB_PASSWORD"); ok && v != "" {
			password = v
		}
	}
	if password == "" {
		return nil, errors.New("DB_PASSWORD 为空，无法执行备份（fail-closed：拒绝在无凭据下执行含 DDL 的迁移）")
	}
	return &mysqlBackupCfg{User: user, Password: password, Host: host, Port: port, DBName: dbname}, nil
}

func fileExists(p string) bool {
	if _, err := os.Stat(p); err == nil {
		return true
	}
	return false
}

// envValue 从 key=value 形式的 env 文件读取指定键的值。
func envValue(path, key string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for _, line := range splitLines(string(data)) {
		line = trimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		if idx := indexOf(line, "="); idx > 0 && trimSpace(line[:idx]) == key {
			return trimSpace(line[idx+1:]), true
		}
	}
	return "", false
}

// backupDatabaseBeforeDDL 执行 mysqldump | zstd 备份（fail-closed）。
// 仅 MySQL 生产路径调用；SQLite 测试由调用方（MigrateDatabaseVersioned）跳过。
func backupDatabaseBeforeDDL(db *gorm.DB) error {
	creds, err := resolveBackupCreds()
	if err != nil {
		return err
	}
	backupDir := backupTargetDir()
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return fmt.Errorf("创建备份目录 %s 失败: %w", backupDir, err)
	}
	ts := time.Now().UTC().Format("20060102_150405")
	outFile := filepath.Join(backupDir, fmt.Sprintf("tsloms_%s_%s.sql.zst", creds.DBName, ts))

	cmdStr := fmt.Sprintf(
		"mysqldump --single-transaction --set-gtid-purged=OFF -u '%s' -p'%s' -h '%s' -P '%s' %s | zstd -q -o %s",
		creds.User, creds.Password, creds.Host, creds.Port, creds.DBName, outFile)

	var cmd *exec.Cmd
	if commandExists("sh") {
		cmd = exec.Command("sh", "-c", cmdStr)
	} else {
		// Windows 兜底（一般不用于生产迁移备份，此处保证可编译）
		return fmt.Errorf("生产备份需 sh/mysqldump/zstd，当前环境不支持")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("备份命令执行失败: %v; 输出: %s", err, string(out))
	}
	if !fileExists(outFile) {
		return fmt.Errorf("备份命令未生成文件 %s", outFile)
	}
	log.Printf("[TSLOMS][migration] 迁移前备份成功 -> %s", outFile)
	return nil
}

// backupTargetDir 备份根目录：优先 releases/backups/db，生产兜底 /opt/tsloms/backups/db。
func backupTargetDir() string {
	rel := filepath.Join("releases", "backups", "db")
	if abs, err := filepath.Abs(rel); err == nil {
		return abs
	}
	return "/opt/tsloms/backups/db"
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// —— 轻量字符串工具（避免为 env 解析引入额外依赖）——

func splitLines(s string) []string {
	var out []string
	line := ""
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			if line != "" {
				out = append(out, line)
				line = ""
			}
			continue
		}
		line += string(s[i])
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end {
		c := s[start]
		if c == ' ' || c == '\t' {
			start++
			continue
		}
		break
	}
	for end > start {
		c := s[end-1]
		if c == ' ' || c == '\t' {
			end--
			continue
		}
		break
	}
	return s[start:end]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
