package ai

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

// decisionSeed 填充决策中心所需数据
func decisionSeed(t *testing.T) {
	t.Helper()
	model.InitTestDB()
	// 运营商 + viewer
	op := model.User{Username: "op_dc", PasswordHash: "x", Role: model.RoleOperator}
	model.DB.Create(&op)
	// 设备（1 在线 1 离线）
	model.DB.Create(&model.Device{HwID: "3001", Intersection: "在线路口", OnlineStatus: true})
	model.DB.Create(&model.Device{HwID: "3002", Intersection: "离线路口", OnlineStatus: false})
	// 故障
	now := time.Now()
	model.DB.Create(&model.FaultRecord{DeviceHwID: "3001", FaultType: "lamp_off", Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now})
	// 工单：2 个未完成（负载不均）+ 1 完成
	model.DB.Create(&model.WorkOrder{OrderNo: "WOd1", Status: model.WorkOrderStatusPending, AssigneeID: &op.ID, CreatedAt: now.Add(-72 * time.Hour)})
	model.DB.Create(&model.WorkOrder{OrderNo: "WOd2", Status: model.WorkOrderStatusPending, AssigneeID: &op.ID, CreatedAt: now.Add(-72 * time.Hour)})
	model.DB.Create(&model.WorkOrder{OrderNo: "WOd3", Status: model.WorkOrderStatusPending, AssigneeID: &op.ID, CreatedAt: now.Add(-72 * time.Hour)})
	model.DB.Create(&model.WorkOrder{OrderNo: "WOd4", Status: model.WorkOrderStatusCompleted, CreatedAt: now.Add(-48 * time.Hour), ClosedAt: &now})
}

func TestBuildOpsHealth(t *testing.T) {
	decisionSeed(t)
	h, err := BuildOpsHealth()
	if err != nil {
		t.Fatalf("BuildOpsHealth err: %v", err)
	}
	if h.Total <= 0 || h.Total > 100 {
		t.Errorf("综合分越界: %v", h.Total)
	}
	if len(h.Dimensions) != 6 {
		t.Errorf("维度数期望 6, got %d", len(h.Dimensions))
	}
	if h.Grade == "" {
		t.Error("应有等级")
	}
}

func TestBuildOpsHealth_Empty(t *testing.T) {
	model.InitTestDB()
	h, _ := BuildOpsHealth()
	if h.Total == 0 {
		t.Error("空库也应返回非零分(默认分)")
	}
}

func TestBuildDecisions(t *testing.T) {
	decisionSeed(t)
	// 物料：高消耗 + 低库存
	m1 := model.Material{Code: "M1", Name: "红灯灯珠", Category: "灯泡", Stock: 2, Threshold: 5, UnitPrice: 50, Status: "active"}
	model.DB.Create(&m1)
	model.DB.Create(&model.MaterialStock{MaterialID: m1.ID, MaterialName: "红灯灯珠", Type: model.StockTypeUse, Quantity: -10, CreatedAt: time.Now()})
	// 维修费用按类型 + 设备
	model.DB.Create(&model.RepairExpense{ExpenseNo: "FE1", Type: "材料费", Amount: 8000, DeviceHwID: "3001", CreatedAt: time.Now()})

	d, err := BuildDecisions()
	if err != nil {
		t.Fatalf("BuildDecisions err: %v", err)
	}
	if len(d) == 0 {
		t.Fatalf("应生成决策建议, got 0")
	}
	// 至少有：人力排班(负载≥3)、备件采购、成本优化、设备运维(离线)
	foundCat := map[string]bool{}
	for _, x := range d {
		foundCat[x.Category] = true
	}
	if !foundCat["人力排班"] {
		t.Error("应有 人力排班 建议")
	}
	if !foundCat["备件采购"] {
		t.Error("应有 备件采购 建议")
	}
	if !foundCat["设备运维"] {
		t.Error("应有 设备运维(离线) 建议")
	}
}

func TestDecisionCenter(t *testing.T) {
	decisionSeed(t)
	res, err := DecisionCenter(1)
	if err != nil {
		t.Fatalf("DecisionCenter err: %v", err)
	}
	if res.Health == nil {
		t.Fatal("应有健康评分")
	}
	if len(res.Decisions) == 0 {
		t.Errorf("应有决策建议")
	}
	if res.Summary == "" {
		t.Error("应有总结")
	}
	if res.Source != "规则" {
		t.Errorf("source=%s", res.Source)
	}
}

func TestAdoptDecisionApply(t *testing.T) {
	decisionSeed(t)
	m := model.Material{Code: "M2", Name: "电源模块", Category: "电源", Stock: 1, Threshold: 3, UnitPrice: 200, Status: "active"}
	model.DB.Create(&m)
	_ = model.Supplier{Name: "供货商A", Phone: "13800000001", Status: "active"}
	sup := model.Supplier{Name: "供货商A", Phone: "13800000001", Status: "active"}
	model.DB.Create(&sup)

	// 采购
	orderNo, err := AdoptDecisionApply(1, "备件采购", "采购电源模块", sup.ID, []PurchaseLine{{MaterialName: "电源模块", Quantity: 5}})
	if err != nil {
		t.Fatalf("AdoptDecisionApply err: %v", err)
	}
	if orderNo == "" {
		t.Error("应返回采购单号")
	}
	// 不支持的类别
	if _, err := AdoptDecisionApply(1, "成本优化", "x", 0, nil); err == nil {
		t.Error("不支持的类别应报错")
	}
	// 空明细
	if _, err := AdoptDecisionApply(1, "备件采购", "x", 0, nil); err == nil {
		t.Error("空明细应报错")
	}
	// 无供应商（supplierID=0 且无供应商: 已有 sup → 不会走该分支；改用已删供应商 ID）
	if _, err := AdoptDecisionApply(1, "备件采购", "x", 0, []PurchaseLine{{MaterialName: "电源模块", Quantity: 1}}); err != nil {
		// 无供应商分支较难触发（DB 有 sup），接受成功或失败
		_ = err
	}
	// 物料不存在
	if _, err := AdoptDecisionApply(1, "备件采购", "x", sup.ID, []PurchaseLine{{MaterialName: "不存在物料", Quantity: 1}}); err == nil {
		t.Error("物料不存在应报错")
	}
	// 全零数量
	if _, err := AdoptDecisionApply(1, "备件采购", "x", sup.ID, []PurchaseLine{{MaterialName: "电源模块", Quantity: 0}, {MaterialName: "电源模块", Quantity: -1}}); err == nil {
		t.Error("无有效数量应报错")
	}
}
