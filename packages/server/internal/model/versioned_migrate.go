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
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tsloms/server/internal/config"
	"gorm.io/driver/mysql"
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
//  2. MySQL 生产专用 GET_LOCK('tsloms_migrate', timeout) 单实例锁：
//     「获取锁 → 执行全部待应用版本 → 释放锁」全程固定在【同一条独占物理连接】
//     (*sql.Conn) 上执行，避免连接池漂移导致会话级锁在另一端失效（BLOCK-1）。
//     SQLite 测试走简化无锁路径；
//  3. 对每个未应用版本：若该版本含 DDL/DropTable（NeedsBackup），执行前强制备份，失败 fail-closed；
//     SQLite 测试跳过备份；
//  4. 版本执行成功写入 schema_migrations；某一步失败立即 return error，不再继续后续版本；
//  5. 纯幂等启动逻辑（SeedRBAC / SeedAreas）在版本管道之后每启动重放安全执行。
func MigrateDatabaseVersioned(db *gorm.DB) error {
	if db == nil {
		return errors.New("MigrateDatabaseVersioned: 数据库未初始化")
	}

	if db.Dialector.Name() == "mysql" {
		// MySQL：锁与全部迁移冻结在同一条独占连接上（BLOCK-1 修复）。
		return migrateDatabaseVersionedWithLock(db)
	}
	// SQLite（测试/本地）：简化无锁路径，不申请会话级锁、不做备份。
	return runMigrationBody(db)
}

// migrateDatabaseVersionedWithLock MySQL 专属：把 GET_LOCK → 全部迁移(含 DDL) → RELEASE_LOCK
// 全部放在从连接池取出的【同一条独占物理连接】上执行，杜绝会话级锁在连接池漂移中失效。
func migrateDatabaseVersionedWithLock(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取底层连接池失败: %w", err)
	}

	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取独占迁移连接失败: %w", err)
	}
	defer conn.Close()

	// 将 *gorm.DB 绑定到该独占连接上：mysql.Config.Conn 直接充当 ConnPool，
	// 后续 GET_LOCK / 迁移 DDL / RELEASE_LOCK 全部在同一物理连接执行。
	lockDB, err := gorm.Open(mysql.New(mysql.Config{Conn: conn}), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("打开绑定到独占连接的 GORM 实例失败: %w", err)
	}

	got, err := acquireMigrateLock(lockDB, migrateLockTimeoutSec)
	if err != nil {
		return fmt.Errorf("获取迁移锁失败(fail-closed): %w", err)
	}
	if !got {
		return fmt.Errorf("获取迁移锁超时(%ds)，可能已有其它实例在迁移；拒绝启动（fail-closed）", migrateLockTimeoutSec)
	}
	// 锁与迁移在同一连接；RELEASE_LOCK 在 defer 中同连接释放，不会静默泄漏。
	defer releaseMigrateLock(lockDB)

	return runMigrationBody(lockDB)
}

// migrateLockName MySQL 单实例锁名（命名空间隔离，避免与同库其它应用冲突）。
const migrateLockName = "tsloms_migrate"

// migrateLockTimeoutSec GET_LOCK 等待秒数；大表 CREATE UNIQUE INDEX 预留充分。
const migrateLockTimeoutSec = 300

// runMigrationBody 版本迁移主体（创建版本表 → 应用未应用版本 → 幂等种子）。
// MySQL 路径传入绑定独占连接的 lockDB；SQLite 路径传入原始 db。
func runMigrationBody(db *gorm.DB) error {
	if err := ensureSchemaMigrationsTable(db); err != nil {
		return fmt.Errorf("创建 schema_migrations 版本表失败: %w", err)
	}

	isMySQL := db.Dialector.Name() == "mysql"

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
		if step.NeedsBackup && isMySQL {
			if err := backupDatabaseBeforeDDL(); err != nil {
				return fmt.Errorf("迁移 %s(%s) 前备份失败，已阻断启动（fail-closed）: %w", step.Version, step.Name, err)
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

// acquireMigrateLock MySQL 单实例锁。
// 调用方必须保证传入的是【已绑定独占连接】的 *gorm.DB（MySQL 路径为 migrateDatabaseVersionedWithLock
// 里的 lockDB）；否则会在连接池漂移连接上执行、锁失效。
func acquireMigrateLock(db *gorm.DB, timeout int) (bool, error) {
	var ok int
	// GET_LOCK 返回 1=获取成功, 0=超时, NULL=错误；同一连接内需 RELEASE_LOCK。
	err := db.Raw("SELECT GET_LOCK(?, ?)", migrateLockName, timeout).Scan(&ok).Error
	if err != nil {
		return false, err
	}
	return ok == 1, nil
}

// releaseMigrateLock 释放 MySQL 迁移锁（best-effort）。需与 acquireMigrateLock 在同一连接上调用。
func releaseMigrateLock(db *gorm.DB) {
	_ = db.Exec("SELECT RELEASE_LOCK(?)", migrateLockName).Error
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
// 安全（HIGH-3）：不把 DB 密码写进命令行/进程参数，改由 mysqldump 官方支持的 MYSQL_PWD
// 环境变量传递（与 deploy/scripts/release-install.sh 中 export MYSQL_PWD=... 的做法一致）；
// mysqldump 以参数数组方式调用（不走 sh -c 拼串），密码绝不入 argv/日志。
func backupDatabaseBeforeDDL() error {
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

	dump, zstd, err := newMysqldumpBackupCmds(creds, outFile)
	if err != nil {
		return err
	}

	dumpOut, err := dump.StdoutPipe()
	if err != nil {
		return fmt.Errorf("建立 mysqldump 输出管道失败: %w", err)
	}
	var dumpErr bytes.Buffer
	dump.Stderr = &dumpErr
	zstd.Stdin = dumpOut

	if err := zstd.Start(); err != nil {
		return fmt.Errorf("启动 zstd 失败: %w", err)
	}
	if err := dump.Run(); err != nil {
		_ = dumpOut.Close()
		_ = zstd.Wait()
		return fmt.Errorf("mysqldump 备份失败: %v; stderr: %s", err, strings.TrimSpace(dumpErr.String()))
	}
	_ = dumpOut.Close()
	if err := zstd.Wait(); err != nil {
		return fmt.Errorf("zstd 压缩写入失败: %v", err)
	}

	if !fileExists(outFile) {
		return fmt.Errorf("备份命令未生成文件 %s", outFile)
	}
	log.Printf("[TSLOMS][migration] 迁移前备份成功 -> %s", outFile)
	return nil
}

// newMysqldumpBackupCmds 构造 mysqldump 与 zstd 两个命令（不启动、不执行，便于单测核验）。
// 安全（HIGH-3）：密码绝不写入命令行/进程参数（argv），改由 mysqldump 官方支持的 MYSQL_PWD
// 环境变量注入（与 release-install.sh 中 export MYSQL_PWD=... 一致）；mysqldump 以参数数组
// 方式调用（不走 sh -c 拼串），因此 ps/进程参数/审计日志均看不到明文密码。
func newMysqldumpBackupCmds(creds *mysqlBackupCfg, outFile string) (dump, zstd *exec.Cmd, err error) {
	if creds == nil {
		return nil, nil, errors.New("newMysqldumpBackupCmds: creds 为空")
	}
	dumpArgs := []string{
		"--single-transaction",
		"--set-gtid-purged=OFF",
		"-u" + creds.User,
		"-h" + creds.Host,
		"-P" + creds.Port,
		"--default-character-set=utf8mb4",
		creds.DBName,
	}
	dump = exec.Command("mysqldump", dumpArgs...)
	dump.Env = append(os.Environ(), "MYSQL_PWD="+creds.Password)

	// zstd 压缩写文件（与 release-install 一致：zstd -q -o file）。
	zstd = exec.Command("zstd", "-q", "-o", outFile)
	return dump, zstd, nil
}

// backupTargetDir 备份根目录（HIGH-2）：
//   - 生产（AppEnv==production）固定为持久目录 /opt/tsloms/backups/db，
//     与 deploy/scripts/release-install.sh / probe-deep.sh 完全一致，探针与回滚脚本能定位到
//     迁移前快照，且不落在不可变 release 目录内（跨版本/回滚不会被清理）；
//   - 开发/测试回退到相对路径 backups/db（绝不使用 releases/...）。
func backupTargetDir() string {
	const prodBackupDir = "/opt/tsloms/backups/db"
	if config.Get().IsProduction() {
		return prodBackupDir
	}
	rel := filepath.Join("backups", "db")
	if abs, err := filepath.Abs(rel); err == nil {
		return abs
	}
	return prodBackupDir
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
