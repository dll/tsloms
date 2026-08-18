package ai

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

// seedBasicData 写入基础测试数据（物料/库存流水/费用/设备/故障/工单）
func seedBasicData(t *testing.T) {
	t.Helper()
	model.InitTestDB()

	// 物料
	model.DB.Create(&model.Material{Code: "BL001", Name: "红灯灯珠", Category: "灯泡",
		Unit: "个", UnitPrice: 25, Stock: 20, Threshold: 10, Status: "active"})
	model.DB.Create(&model.Material{Code: "PS001", Name: "电源模块", Category: "电源",
		Unit: "个", UnitPrice: 200, Stock: 2, Threshold: 5, Status: "active"}) // 低库存
	model.DB.Create(&model.Material{Code: "ST001", Name: "闲置线缆", Category: "线缆",
		Unit: "米", UnitPrice: 3, Stock: 100, Threshold: 0, Status: "active"}) // 滞销

	// 设备 + 故障
	model.DB.Create(&model.Device{HwID: "1001", Intersection: "人民路口", OnlineStatus: true})
	model.DB.Create(&model.Device{HwID: "1002", Intersection: "建设路口", OnlineStatus: false})
	now := time.Now()
	model.DB.Create(&model.FaultRecord{DeviceHwID: "1001", ErrCode: -1, FaultType: "lamp_off",
		FaultLevel: "critical", Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now})

	// 工单 + 领料流水 + 费用
	wo := model.WorkOrder{OrderNo: "WO202608150001", FaultID: 1, DeviceHwID: "1001",
		Status: model.WorkOrderStatusCompleted, CreatedAt: now.Add(-48 * time.Hour)}
	model.DB.Create(&wo)
	closed := now.Add(-24 * time.Hour)
	model.DB.Model(&wo).Updates(map[string]any{"closed_at": &closed})

	model.DB.Create(&model.MaterialStock{MaterialID: 1, MaterialName: "红灯灯珠", Type: model.StockTypeUse,
		Quantity: -2, Price: 25, Amount: -50, RefType: "repair", WorkOrderID: &wo.ID, CreatedAt: now.Add(-24 * time.Hour)})
	model.DB.Create(&model.MaterialStock{MaterialID: 1, MaterialName: "红灯灯珠", Type: model.StockTypeIn,
		Quantity: 20, Price: 25, Amount: 500, RefType: "purchase", CreatedAt: now.Add(-48 * time.Hour)})

	model.DB.Create(&model.RepairExpense{ExpenseNo: "FE202608150001", WorkOrderID: &wo.ID,
		DeviceHwID: "1001", Type: model.ExpenseTypeMaterial, Amount: 50, Confirmed: true, CreatedAt: now.Add(-24 * time.Hour)})
	model.DB.Create(&model.RepairExpense{ExpenseNo: "FE202608150002",
		DeviceHwID: "1001", Type: model.ExpenseTypeLabor, Amount: 120, Confirmed: false, CreatedAt: now.Add(-24 * time.Hour)})
}

func TestBuildInventorySnapshot(t *testing.T) {
	seedBasicData(t)
	snap, err := BuildInventorySnapshot()
	if err != nil {
		t.Fatalf("BuildInventorySnapshot err: %v", err)
	}
	if snap.TotalKinds != 3 {
		t.Errorf("TotalKinds=%d, want 3", snap.TotalKinds)
	}
	if len(snap.LowStock) < 1 {
		t.Errorf("应检测到低库存物料(电源模块), got %d", len(snap.LowStock))
	}
	if len(snap.SlowMoving) < 1 {
		t.Errorf("应检测到滞销物料(闲置线缆), got %d", len(snap.SlowMoving))
	}
	if snap.RecentUse < 2 {
		t.Errorf("近30天领用应≥2, got %d", snap.RecentUse)
	}
}

func TestBuildCostSnapshot(t *testing.T) {
	seedBasicData(t)
	snap, err := BuildCostSnapshot(90)
	if err != nil {
		t.Fatalf("BuildCostSnapshot err: %v", err)
	}
	if snap.TotalAmount < 170 {
		t.Errorf("总成本应≥170(耗材50+人工120), got %.2f", snap.TotalAmount)
	}
	if len(snap.TopDevices) < 1 {
		t.Errorf("应统计到高成本设备, got %d", len(snap.TopDevices))
	}
	if snap.Confirmed < 50 || snap.Unconfirmed < 120 {
		t.Errorf("已确认/未确认金额异常: confirmed=%.2f unconfirmed=%.2f", snap.Confirmed, snap.Unconfirmed)
	}
}

func TestBuildDailySnapshot(t *testing.T) {
	seedBasicData(t)
	snap, err := BuildDailySnapshot()
	if err != nil {
		t.Fatalf("BuildDailySnapshot err: %v", err)
	}
	if snap.WorkOrders.Total < 1 {
		t.Errorf("应统计到工单, got %d", snap.WorkOrders.Total)
	}
	if snap.Faults.Active < 1 {
		t.Errorf("应有活跃故障, got %d", snap.Faults.Active)
	}
}

// 纯函数测试：规则文案生成
func TestBuildInventoryRuleInsight(t *testing.T) {
	s := &InventorySnapshot{
		OutOfStock: []MaterialMin{{Name: "红灯灯珠"}},
		LowStock:   []MaterialMin{{Name: "电源模块"}},
	}
	insight := buildInventoryRuleInsight(s)
	if insight == "" {
		t.Fatal("规则洞察不应为空")
	}
}

func TestTypeLabel(t *testing.T) {
	if typeLabel(model.ExpenseTypeMaterial) != "耗材" {
		t.Errorf("typeLabel(material) 应为 耗材")
	}
	if typeLabel(model.ExpenseTypeLabor) != "人工" {
		t.Errorf("typeLabel(labor) 应为 人工")
	}
}

func TestExtractAdv(t *testing.T) {
	text := "故障摘要：某路口红灯故障\n优先级：P1\n应对预案：检查供电\n建议备件：红灯灯珠"
	if v := extractAdv(text, "故障摘要"); v != "某路口红灯故障" {
		t.Errorf("extractAdv 摘要=%q", v)
	}
	if v := extractAdv(text, "优先级"); v != "P1" {
		t.Errorf("extractAdv 优先级=%q", v)
	}
	parts := extractAdvList(text, "建议备件")
	if len(parts) != 1 || parts[0] != "红灯灯珠" {
		t.Errorf("extractAdvList 备件=%v", parts)
	}
}
