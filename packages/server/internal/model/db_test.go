package model

import (
	"testing"
	"time"
)

// TestMigrateLegacyDeviceMaterials 验证旧设备耗材台账并入统一物料档案
// 覆盖：新建物料+初始库存流水、同名物料跳过、设备绑定补充、空表删除旧表
func TestMigrateLegacyDeviceMaterials(t *testing.T) {
	db := InitTestDB()

	// 构造旧表（inline 方式创建，避免依赖已删除的 DeviceMaterial 模型）
	if err := db.Exec(`CREATE TABLE device_materials (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_hw_id INTEGER,
		name TEXT,
		part_no TEXT,
		spec TEXT,
		quantity INTEGER DEFAULT 0,
		unit TEXT,
		threshold INTEGER DEFAULT 0,
		note TEXT,
		last_changed_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}

	now := time.Now()
	// 3 条：两条新物料（不同编码），一条与已存在物料重名
	legacy := []struct {
		DeviceHwID uint32
		Name       string
		PartNo     string
		Spec       string
		Unit       string
		Note       string
		Quantity   int
		Threshold  int
	}{
		{DeviceHwID: 1, Name: "中文测试黄灯珠", PartNo: "", Spec: "黄色", Quantity: 5, Unit: "支", Threshold: 0, Note: "旧数据A"},
		{DeviceHwID: 1, Name: "LED驱动电源", PartNo: "PS-48V", Spec: "48V", Quantity: 6, Unit: "个", Threshold: 2, Note: "旧数据B"},
		{DeviceHwID: 2, Name: "已存在物料X", PartNo: "EXIST-X", Spec: "", Quantity: 8, Unit: "个", Threshold: 0, Note: "重复"},
	}
	for _, r := range legacy {
		if err := db.Exec(`INSERT INTO device_materials (device_hw_id,name,part_no,spec,quantity,unit,threshold,note,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			r.DeviceHwID, r.Name, r.PartNo, r.Spec, r.Quantity, r.Unit, r.Threshold, r.Note, now, now).Error; err != nil {
			t.Fatalf("insert legacy: %v", err)
		}
	}
	if err := db.Create(&Material{Code: "EXIST-X", Name: "已存在物料X", Category: "其他", Stock: 20, Threshold: 0, Status: "active"}).Error; err != nil {
		t.Fatalf("pre-create material: %v", err)
	}

	if err := MigrateLegacyDeviceMaterials(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 旧表应已被删除
	if db.Migrator().HasTable("device_materials") {
		t.Errorf("legacy table should be dropped")
	}

	// 新增了 2 条新物料（重名那条跳过，但设备绑定应补充到 EXIST-X）
	var mats []Material
	if err := db.Order("id ASC").Find(&mats).Error; err != nil {
		t.Fatalf("find materials: %v", err)
	}
	if len(mats) != 3 {
		t.Fatalf("expected 3 materials (1 pre + 2 migrated), got %d", len(mats))
	}

	// 检查迁移的两条
	var dby, ps Material
	dby = mats[1] // 中文测试黄灯珠 (part_no为空→用name作code)
	if !(dby.Name == "中文测试黄灯珠" && dby.Code == "中文测试黄灯珠" && dby.Stock == 5) {
		t.Errorf("黄灯珠 migrate wrong: %+v", dby)
	}
	if dby.DeviceHwID == nil || *dby.DeviceHwID != 1 {
		t.Errorf("黄灯珠 device_hw_id not bound: %+v", dby.DeviceHwID)
	}
	ps = mats[2]
	if !(ps.Name == "LED驱动电源" && ps.Code == "PS-48V" && ps.Stock == 6 && ps.Threshold == 2) {
		t.Errorf("电源 migrate wrong: %+v", ps)
	}
	if ps.DeviceHwID == nil || *ps.DeviceHwID != 1 {
		t.Errorf("电源 device_hw_id not bound: %+v", ps.DeviceHwID)
	}

	// 同名物料 EXIST-X 设备绑定应补充为设备2，且库存保持 20 不变
	var exist Material
	if err := db.Where("code = ?", "EXIST-X").First(&exist).Error; err != nil {
		t.Fatalf("find EXIST-X: %v", err)
	}
	if exist.DeviceHwID == nil || *exist.DeviceHwID != 2 {
		t.Errorf("EXIST-X device binding not supplemented: %+v", exist.DeviceHwID)
	}
	if exist.Stock != 20 {
		t.Errorf("EXIST-X stock should stay 20, got %d", exist.Stock)
	}

	// 初始库存流水：迁移的 5+6=11 条（quantity=5 与 quantity=6），EXIST-X 不追加
	var stocks []MaterialStock
	if err := db.Find(&stocks).Error; err != nil {
		t.Fatalf("find stocks: %v", err)
	}
	legacyStock := 0
	for _, s := range stocks {
		if s.Note == "旧耗材台账合并初始库存" {
			legacyStock++
		}
	}
	if legacyStock != 2 {
		t.Errorf("expected 2 legacy initial stock records, got %d", legacyStock)
	}
}

// TestMigrateLegacyDeviceMaterialsNoData 旧表无数据时应被直接删除
func TestMigrateLegacyDeviceMaterialsNoData(t *testing.T) {
	db := InitTestDB()
	if err := db.Exec(`CREATE TABLE device_materials (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_hw_id INTEGER, name TEXT, part_no TEXT, spec TEXT,
		quantity INTEGER, unit TEXT, threshold INTEGER, note TEXT,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := MigrateLegacyDeviceMaterials(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if db.Migrator().HasTable("device_materials") {
		t.Errorf("empty legacy table should be dropped")
	}
}
