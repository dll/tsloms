package model

// qa_versioned_migrate_regression_test.go —— CD-P0-01 回归守护用例（QA 新增）。
//
// 与 versioned_migrate_test.go（dev 自带）互补但不重复：本文件聚焦「回归守护」视角，
// 验证重构后的显式版本化迁移在三个维度不使原有业务退化：
//   1. 二次执行严格幂等：第二次执行无副作用（迁移计数不再增长、业务数据不变、版本不重复应用）；
//   2. schema_migrations 版本表 applied 记录字段完整（applied_by / version / applied_at / name）；
//   3. 全新库一键迁移（FreshDB）后 38 张业务表全量可达（比 dev 抽样更广，逐张断言建表）；
//   4. 业务红线：InitTestDB() 一键全量建表+种子 语义与重构前一致（RBAC 种子 / 超管 / 区划俱在）。

import (
	"fmt"
	"testing"

	"gorm.io/gorm"
)

// qaAllBusinessTables 全量业务表模型清单（38 张），用于 FreshDB 一键建表完整性回归。
var qaAllBusinessTables = []interface{}{
	&Device{}, &PacketLog{}, &FaultRecord{}, &FaultEvidence{}, &FaultCase{},
	&WorkOrder{}, &User{}, &Department{}, &OperationLog{}, &DeviceMedia{},
	&Feedback{}, &AIConfig{}, &AIUsage{}, &AIPrediction{}, &FirmwarePackage{},
	&FirmwareUpgradeRecord{}, &Material{}, &MaterialStock{}, &Supplier{}, &PurchaseOrder{},
	&PurchaseOrderItem{}, &RepairExpense{}, &AIReport{}, &AIAdvice{}, &Permission{},
	&Role{}, &RolePermission{}, &UserPermission{}, &Notification{}, &NotificationRead{},
	&Warning{}, &WarningRule{}, &Area{}, &Crossing{}, &PatrolTask{},
	&PatrolRecord{}, &ModuleToggle{}, &LicenseState{},
}

// qaCountVersions 统计 schema_migrations 版本表的已应用版本数（不引 db 状态，纯读）。
func qaCountVersions(db *gorm.DB) (int, error) {
	var cnt int64
	if err := db.Model(&schemaMigrations{}).Count(&cnt).Error; err != nil {
		return 0, err
	}
	return int(cnt), nil
}

// tableNameOf 解析模型表名（仅供失败信息可读性；实际建表断言用 HasTable）。
// 优先尝试 GORM NamingStrategy 默认命名；自定义 TableName 方法将不在此解析，但仅在失败时用于提示。
func tableNameOf(db *gorm.DB, m interface{}) string {
	tn := db.NamingStrategy.TableName(simpleTypeName(m))
	if tn != "" {
		return tn
	}
	return fmt.Sprintf("%T", m)
}

// simpleTypeName 取模型底层结构体名（去包前缀、去指针）。
func simpleTypeName(m interface{}) string {
	raw := fmt.Sprintf("%T", m)
	if len(raw) > 1 && raw[0] == '*' {
		raw = raw[1:]
	}
	for i := len(raw) - 1; i >= 0; i-- {
		if raw[i] == '.' {
			return raw[i+1:]
		}
	}
	return raw
}

// TestQA_VersionedMigrate_IdempotentNoSideEffect
// 回归守护①：二次执行无副作用——迁移版本数不再增长、超管不被重复创建、区划种子不重复。
func TestQA_VersionedMigrate_IdempotentNoSideEffect(t *testing.T) {
	db := InitTestDB()

	before, err := qaCountVersions(db)
	if err != nil {
		t.Fatalf("首次执行后读取版本数: %v", err)
	}
	if before == 0 {
		t.Fatal("首次执行后版本数不应为 0")
	}

	var saBefore, areaBefore, roleBefore int64
	db.Model(&User{}).Where("username = ?", SuperAdminUsername).Count(&saBefore)
	db.Model(&Area{}).Count(&areaBefore)
	db.Model(&Role{}).Count(&roleBefore)

	// 二次执行完整版本化迁移
	if err := MigrateDatabaseVersioned(db); err != nil {
		t.Fatalf("二次执行应幂等成功: %v", err)
	}

	after, err := qaCountVersions(db)
	if err != nil {
		t.Fatalf("二次执行后读取版本数: %v", err)
	}
	if after != before {
		t.Fatalf("二次执行后版本数 = %d, 应仍为 %d（版本被重复应用）", after, before)
	}

	// 业务数据无副作用
	var saNow, areaNow, roleNow int64
	db.Model(&User{}).Where("username = ?", SuperAdminUsername).Count(&saNow)
	db.Model(&Area{}).Count(&areaNow)
	db.Model(&Role{}).Count(&roleNow)
	if saNow != saBefore {
		t.Errorf("二次执行后超管 = %d, 应仍为 %d（被重复创建）", saNow, saBefore)
	}
	if areaNow != areaBefore {
		t.Errorf("二次执行后区划 = %d, 应仍为 %d（区划种子被重复写入）", areaNow, areaBefore)
	}
	if roleNow != roleBefore {
		t.Errorf("二次执行后角色 = %d, 应仍为 %d", roleNow, roleBefore)
	}
}

// TestQA_VersionedMigrate_AppliedRecordsComplete
// 回归守护②：版本表 applied 记录字段完整（version/applied_at/applied_by/name），且版本唯一。
func TestQA_VersionedMigrate_AppliedRecordsComplete(t *testing.T) {
	db := InitTestDB()

	var rows []schemaMigrations
	if err := db.Order("version ASC").Find(&rows).Error; err != nil {
		t.Fatalf("读取全部版本记录: %v", err)
	}
	if len(rows) != len(orderedMigrations) {
		t.Fatalf("版本记录数 = %d, 期望 %d", len(rows), len(orderedMigrations))
	}

	seen := map[string]bool{}
	for i, r := range rows {
		if r.Version == "" || r.Name == "" || r.AppliedAt == "" || r.AppliedBy == "" {
			t.Errorf("第 %d 条版本记录字段不完整: version=%q name=%q applied_at=%q applied_by=%q",
				i, r.Version, r.Name, r.AppliedAt, r.AppliedBy)
		}
		if seen[r.Version] {
			t.Errorf("版本 %q 重复出现（version 应唯一）", r.Version)
		}
		seen[r.Version] = true
		// 与有序迁移定义对齐
		if r.Version != orderedMigrations[i].Version {
			t.Errorf("版本记录顺序 %q 与定义 %q 不一致", r.Version, orderedMigrations[i].Version)
		}
	}
}

// TestQA_VersionedMigrate_FreshDB_All38Tables
// 回归守护③：全新库一键迁移后，38 张业务表全部建表（比 dev 抽样式更全量逐张断言）。
func TestQA_VersionedMigrate_FreshDB_All38Tables(t *testing.T) {
	db := InitTestDB()
	if len(qaAllBusinessTables) != 38 {
		t.Fatalf("业务表清单数量 = %d, 期望 38（请同步维护清单）", len(qaAllBusinessTables))
	}
	missing := []string{}
	for _, m := range qaAllBusinessTables {
		if !db.Migrator().HasTable(m) {
			missing = append(missing, tableNameOf(db, m))
		}
	}
	if len(missing) != 0 {
		t.Errorf("以下业务表未在版本化迁移中建表: %v", missing)
	}
	// 版本表自身也应存在
	if !db.Migrator().HasTable(&schemaMigrations{}) {
		t.Error("schema_migrations 版本表应存在")
	}
}

// TestQA_InitTestDB_SeedSemanticsUnchanged
// 回归守护④（业务红线）：InitTestDB() 一键全量建表+种子语义与重构前一致。
// 80+ 处测试依赖该语义；这里直接断言 RBAC 种子、超管首建、区划种子在 InitTestDB 后即存在。
func TestQA_InitTestDB_SeedSemanticsUnchanged(t *testing.T) {
	db := InitTestDB()

	// 权限字典种子
	var permCount int64
	db.Model(&Permission{}).Count(&permCount)
	if permCount != int64(len(AllPermissions)) {
		t.Fatalf("权限字典数量 = %d, 期望 %d（InitTestDB 未完成 RBAC 种子）", permCount, len(AllPermissions))
	}

	// 内置角色种子
	var roleCount int64
	db.Model(&Role{}).Count(&roleCount)
	if roleCount != 4 {
		t.Fatalf("内置角色数量 = %d, 期望 4(%s)", roleCount, "super_admin/admin/operator/viewer")
	}

	// 超管首建（不对外开放模块设置）
	if !db.Migrator().HasTable(&User{}) {
		t.Fatal("users 表应已建")
	}
	var saCnt int64
	db.Model(&User{}).Where("username = ?", SuperAdminUsername).Count(&saCnt)
	if saCnt != 1 {
		t.Fatalf("超级管理员 = %d, 期望 1（InitTestDB 未完成超管首建）", saCnt)
	}

	// 区划种子
	var areaCnt int64
	db.Model(&Area{}).Count(&areaCnt)
	if areaCnt == 0 {
		t.Error("区划种子应已写入（InitTestDB 未完成 SeedAreas）")
	}

	// uk_wo_active_scope 唯一索引（M1 并发安全依赖）
	if !db.Migrator().HasIndex(&WorkOrder{}, "uk_wo_active_scope") {
		t.Error("uk_wo_active_scope 唯一索引应已建立（InitTestDB 未完成版本 0002）")
	}
}
