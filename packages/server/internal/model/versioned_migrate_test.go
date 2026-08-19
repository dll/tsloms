package model

import (
	"testing"
)

// TestMigrateDatabaseVersioned_AppliesAndIsIdempotent
// 验证版本化迁移：首次执行应用全部 0001~0004，schema_migrations 版本表正确写入；
// 二次执行幂等跳过（已应用），业务表仍在，超管/RBAC/区划种子俱在。
func TestMigrateDatabaseVersioned_AppliesAndIsIdempotent(t *testing.T) {
	db := InitTestDB() // InitTestDB 内部即走 MigrateDatabaseVersioned（SQLite 简化无锁无备份）

	// 版本表应存在且含全部 0001~0004
	if !db.Migrator().HasTable(&schemaMigrations{}) {
		t.Fatal("schema_migrations 版本表应已创建")
	}
	var applied []string
	if err := db.Model(&schemaMigrations{}).Order("version ASC").Pluck("version", &applied).Error; err != nil {
		t.Fatalf("读取已应用版本: %v", err)
	}
	want := []string{"0001", "0002", "0003", "0004"}
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
