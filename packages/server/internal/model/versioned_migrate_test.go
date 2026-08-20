package model

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tsloms/server/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestMigrateDatabaseVersioned_AppliesAndIsIdempotent
// 验证版本化迁移：首次执行应用全部 0001~0004，schema_migrations 版本表正确写入；
// 二次执行幂等跳过（已应用），业务表仍在，超管/RBAC/区划种子俱在。
func TestMigrateDatabaseVersioned_AppliesAndIsIdempotent(t *testing.T) {
	db := InitTestDB() // InitTestDB 内部即走 MigrateDatabaseVersioned（SQLite 简化无锁无备份）

	// 版本表应存在且含全部已定义版本
	if !db.Migrator().HasTable(&schemaMigrations{}) {
		t.Fatal("schema_migrations 版本表应已创建")
	}
	var applied []string
	if err := db.Model(&schemaMigrations{}).Order("version ASC").Pluck("version", &applied).Error; err != nil {
		t.Fatalf("读取已应用版本: %v", err)
	}
	want := []string{"0001", "0002", "0003", "0004", "0005"}
	if len(applied) != len(want) {
		t.Fatalf("已应用版本数 = %d, 期望 %d; 实际 %v", len(applied), len(want), applied)
	}
	for i := range want {
		if applied[i] != want[i] {
			t.Errorf("版本顺序不符: got %v, want %v", applied, want)
		}
	}

	// 业务前置完整：唯一索引 + 超管 + RBAC
	if !db.Migrator().HasIndex(&WorkOrder{}, "uk_wo_active_scope") {
		t.Error("uk_wo_active_scope 唯一索引应已建立")
	}
	var saCnt int64
	db.Model(&User{}).Where("username = ?", SuperAdminUsername).Count(&saCnt)
	if saCnt != 1 {
		t.Errorf("超级管理员数量 = %d, 期望 1", saCnt)
	}
	var permCount int64
	db.Model(&Permission{}).Count(&permCount)
	if permCount != int64(len(AllPermissions)) {
		t.Errorf("权限字典数量 = %d, 期望 %d", permCount, len(AllPermissions))
	}
	var roleCount int64
	db.Model(&Role{}).Count(&roleCount)
	if roleCount != 4 {
		t.Errorf("内置角色数量 = %d, 期望 4", roleCount)
	}
	var areaCount int64
	db.Model(&Area{}).Count(&areaCount)
	if areaCount == 0 {
		t.Error("区划种子应已写入")
	}

	// 幂等：二次执行应跳过已应用版本，不重复报错、不重建超管
	if err := MigrateDatabaseVersioned(db); err != nil {
		t.Fatalf("二次执行版本化迁移应幂等成功: %v", err)
	}
	var still []string
	db.Model(&schemaMigrations{}).Order("version ASC").Pluck("version", &still)
	if len(still) != len(want) {
		t.Fatalf("幂等执行后版本表被破坏: %v", still)
	}
	db.Model(&User{}).Where("username = ?", SuperAdminUsername).Count(&saCnt)
	if saCnt != 1 {
		t.Errorf("幂等执行后超管应仍是 1 个, 实际 %d", saCnt)
	}
}

// TestMigrateDatabaseVersioned_RecordsAppliedBy 验证版本表记录 applied_at/applied_by。
func TestMigrateDatabaseVersioned_RecordsAppliedBy(t *testing.T) {
	db := InitTestDB()
	var rec schemaMigrations
	if err := db.Where("version = ?", "0002").First(&rec).Error; err != nil {
		t.Fatalf("读取 0002 版本记录: %v", err)
	}
	if rec.Name == "" || rec.AppliedAt == "" || rec.AppliedBy == "" {
		t.Errorf("版本记录字段不完整: %+v", rec)
	}
}

// TestMigrateDatabaseVersioned_FreshDB_OneClick 全新库一键建表+种子（38 表可达性抽样）。
func TestMigrateDatabaseVersioned_FreshDB_OneClick(t *testing.T) {
	db := InitTestDB()
	sample := []interface{}{
		&Device{}, &FaultRecord{}, &WorkOrder{}, &User{}, &Permission{},
		&Role{}, &PatrolTask{}, &PatrolRecord{}, &ModuleToggle{}, &LicenseState{},
		&Material{}, &Area{}, &Crossing{}, &Warning{}, &WarningRule{},
	}
	for _, m := range sample {
		if !db.Migrator().HasTable(m) {
			t.Errorf("表应被版本化迁移创建: %v", m)
		}
	}
}

// setAppEnvInTest 设置 APP_ENV 并重建配置缓存（配合 t.Cleanup 恢复）。
func setAppEnvInTest(t *testing.T, appEnv string) {
	old := os.Getenv("APP_ENV")
	_ = os.Setenv("APP_ENV", appEnv)
	config.ResetCache()
	t.Cleanup(func() {
		if old != "" {
			_ = os.Setenv("APP_ENV", old)
		} else {
			_ = os.Unsetenv("APP_ENV")
		}
		config.ResetCache()
	})
}

// TestBackupTargetDir_PersistentProdPath HIGH-2：生产必须用持久备份目录 /opt/tsloms/backups/db
// （与 release-install.sh / probe-deep.sh 一致），且绝不落回不可变 release 目录。
func TestBackupTargetDir_PersistentProdPath(t *testing.T) {
	// 生产：固定 /opt/tsloms/backups/db
	setAppEnvInTest(t, "production")
	if got := backupTargetDir(); got != "/opt/tsloms/backups/db" {
		t.Errorf("生产备份目录 = %q, 期望 /opt/tsloms/backups/db", got)
	}
}

// TestBackupTargetDir_NonProdNoReleases HIGH-2：开发/测试回退路径不得包含 releases（
// 否则会落进不可变发布目录内、随版本滚动/回滚被清理）。
func TestBackupTargetDir_NonProdNoReleases(t *testing.T) {
	setAppEnvInTest(t, "development")
	got := backupTargetDir()
	if got == "" {
		t.Fatal("开发回退备份目录不应为空")
	}
	if strings.Contains(got, string(filepath.Separator)+"releases") ||
		strings.Contains(got, string(filepath.Separator)+"releases"+string(filepath.Separator)) {
		t.Errorf("开发/测试回退备份目录不应落在 releases 目录内，实际 = %q", got)
	}
}

// TestMysqldumpBackup_NoPlaintextPasswordInArgv HIGH-3：mysqldump 命令参数数组（argv）中
// 不得出现数据库密码明文；密码只经 MYSQL_PWD 环境变量注入（与 release-install.sh 一致）。
func TestMysqldumpBackup_NoPlaintextPasswordInArgv(t *testing.T) {
	creds := &mysqlBackupCfg{
		User:     "tsloms_app",
		Password: "SuperSecret#Password-123",
		Host:     "127.0.0.1",
		Port:     "3306",
		DBName:   "tsloms",
	}
	dump, zstd, err := newMysqldumpBackupCmds(creds, "/opt/tsloms/backups/db/tsloms.sql.zst")
	if err != nil {
		t.Fatalf("构造备份命令失败: %v", err)
	}
	if dump == nil || zstd == nil {
		t.Fatal("两个命令构造应非空")
	}
	// 1) argv 中不得出现密码明文或 -p<密码> / -p'<密码>' 形式
	args := strings.Join(dump.Args, " ")
	if strings.Contains(args, creds.Password) {
		t.Errorf("mysqldump argv 泄漏密码明文: %q", args)
	}
	for _, a := range dump.Args {
		if strings.HasPrefix(a, "-p") || strings.HasPrefix(a, "--password") {
			t.Errorf("mysqldump argv 含密码参数(不应出现 -p/--password): %q", a)
		}
	}
	// 2) 密码以 MYSQL_PWD 环境变量存在
	env := strings.Join(dump.Env, "\n")
	want := "MYSQL_PWD=" + creds.Password
	found := false
	for _, kv := range dump.Env {
		if kv == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("环境变量中未找到 %q；实际 env 片段: %q", want, env)
	}
	// 3) zstd 仅负责压缩落盘，不应携带任何 DB 凭据
	for _, a := range zstd.Args {
		if strings.Contains(a, creds.Password) {
			t.Errorf("zstd argv 泄漏密码: %q", a)
		}
	}
}

// TestMigrateDatabaseVersioned_MySQLGetLockIntegration BLOCK-1：MySQL 集成冒烟。
//
// 本机无 MySQL 时自动跳过（不在 CI/常规单元测试路径执行）；需要集成环境时，通过
// 环境变量 TSLOMS_TEST_MYSQL_DSN 注入真实 DSN（如 'root:pwd@tcp(127.0.0.1:3306)/tsloms_test?parseTime=True'）
// 开启。验证：
//  1. 锁与全部迁移在同一独占连接（GET_LOCK 后迁移后 RELEASE_LOCK 同连接配对，
//     迁移完成后 IS_FREE_LOCK 恢复为 1）；
//  2. 第二实例并发启动时，因 GET_LOCK 超时被 fail-closed 拒绝（不会出现两类并行迁移）。
func TestMigrateDatabaseVersioned_MySQLGetLockIntegration(t *testing.T) {
	dsn := os.Getenv("TSLOMS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未设置 TSLOMS_TEST_MYSQL_DSN，跳过 MySQL 集成验证（BLOCK-1 需在 MySQL 集成环境验证锁配对+并发拒绝+fail-closed）")
	}

	gdb, err := gormOpenForMySQLTest(dsn)
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	defer closeForMySQLTest(gdb)

	// 清理残留，保证从干净库开始验证迁移链路
	if err := resetMySQLTestDB(gdb); err != nil {
		t.Fatalf("清理测试库失败: %v", err)
	}

	// 1) 单实例完整迁移：应在同一条独占连接上 获取锁→迁移→释放锁。
	if err := MigrateDatabaseVersioned(gdb); err != nil {
		t.Fatalf("版本化迁移失败: %v", err)
	}
	// 迁移返回后，defer 已释放锁：IS_FREE_LOCK 应为 1（证明 RELEASE_LOCK 同连接配对成功，未泄漏）。
	if free := mysqlIsFreeLock(gdb); free != 1 {
		t.Errorf("迁移完成后锁应已释放(IS_FREE_LOCK=1)，实际 = %d（锁泄漏!）", free)
	}
	// 版本表记录完整
	var cnt int64
	gdb.Model(&schemaMigrations{}).Count(&cnt)
	if cnt != 4 {
		t.Errorf("schema_migrations 应含 4 条版本记录，实际 = %d", cnt)
	}

	// 2) 幂等：二次执行应跳过已应用版本且不报错。
	if err := MigrateDatabaseVersioned(gdb); err != nil {
		t.Fatalf("二次执行应幂等成功: %v", err)
	}

	// 3) 并发/第二实例 fail-closed：在另一条连接上手动占用同名锁，使 migrateDatabaseVersionedWithLock
	//    的 GET_LOCK 超时返回错误（模拟“已有其它实例在迁移”，拒绝启动）。
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("取底层连接池: %v", err)
	}
	ctx := context.Background()
	blocker, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatalf("取第二连接占用锁: %v", err)
	}
	hold := 0
	if err := blocker.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", migrateLockName).Scan(&hold); err != nil {
		_ = blocker.Close()
		t.Fatalf("第二连接占用锁: %v", err)
	}
	if hold != 1 {
		_ = blocker.Close()
		t.Fatalf("第二连接应成功占用锁, got %d", hold)
	}

	err = migrateDatabaseVersionedWithLock(gdb)
	if err == nil {
		t.Error("第二实例在锁被占用时不应迁移成功（fail-closed 应拒绝）")
	} else if !strings.Contains(err.Error(), "超时") && !strings.Contains(err.Error(), "fail-closed") {
		t.Errorf("第二实例失败原因应为锁超时/fail-closed，实际: %v", err)
	}

	// 释放 blocker 后，锁应可被正常获取（排他性证据）
	_ = blocker.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", migrateLockName).Scan(&hold)
	_ = blocker.Close()
	if free := mysqlIsFreeLock(gdb); free != 1 {
		t.Errorf("释放 blocker 后锁应空闲(IS_FREE_LOCK=1)，实际 = %d", free)
	}
}

// —— 以下为 MySQL 集成测试辅助（BLOCK-1）——

// gormOpenForMySQLTest 按 DSN 打开 MySQL 连接（仅集成测试使用）。
func gormOpenForMySQLTest(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(4) // 多连接，便于验证连接池漂移时锁仍被独占连接保护
	}
	return db, nil
}

func closeForMySQLTest(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// resetMySQLTestDB 清空上一轮集成测试的 schema_migrations 与全部业务表，保证干净基线。
// （返回错误由调用方处理；失败即阻断测试。）
func resetMySQLTestDB(db *gorm.DB) error {
	// 版本化迁移会重建全部 38 表，因此只需清理版本表即可得到“干净待迁移库”。
	if db.Migrator().HasTable(&schemaMigrations{}) {
		if err := db.Migrator().DropTable(&schemaMigrations{}); err != nil {
			return err
		}
	}
	// 同时清理版本化迁移涉及的业务表（uk 索引/旧表等）以免干扰；AutoMigrate 会重建。
	for _, m := range []interface{}{
		&WorkOrder{}, &FaultRecord{}, &Device{}, &User{}, &Permission{}, &Role{},
	} {
		if db.Migrator().HasTable(m) {
			if err := db.Migrator().DropTable(m); err != nil {
				return err
			}
		}
	}
	return nil
}

// mysqlIsFreeLock 查询 IS_FREE_LOCK(<name>)，返回 1=空闲、0=占用。
func mysqlIsFreeLock(db *gorm.DB) int {
	var v int
	_ = db.Raw("SELECT IS_FREE_LOCK(?)", migrateLockName).Scan(&v).Error
	return v
}
