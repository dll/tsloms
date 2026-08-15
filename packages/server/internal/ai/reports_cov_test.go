package ai

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

func reportSeed(t *testing.T) {
	t.Helper()
	model.InitTestDB()
	model.DB.Create(&model.Device{HwID: 1001, Intersection: "人民路口", OnlineStatus: true})
	model.DB.Create(&model.Device{HwID: 1002, Intersection: "建设路口", OnlineStatus: false})
	now := time.Now()
	model.DB.Create(&model.FaultRecord{DeviceHwID: 1001, FaultType: "lamp_off", FaultLevel: "critical", Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now})
	model.DB.Create(&model.FaultRecord{DeviceHwID: 1002, FaultType: "abnormal_on", FaultLevel: "major", Status: model.FaultStatusResolved, FirstSeen: now, LastSeen: now})
	wo := model.WorkOrder{OrderNo: "WO1", Status: model.WorkOrderStatusPending, CreatedAt: now}
	model.DB.Create(&wo)
	model.DB.Create(&model.WorkOrder{OrderNo: "WO2", Status: model.WorkOrderStatusProcessing, CreatedAt: now})
	closed := now.Add(-2 * 24 * time.Hour)
	model.DB.Create(&model.WorkOrder{OrderNo: "WO3", Status: model.WorkOrderStatusCompleted, CreatedAt: closed, ClosedAt: &now})
	model.DB.Create(&model.RepairExpense{ExpenseNo: "FE1", Amount: 1200, CreatedAt: now})
	model.DB.Create(&model.MaterialStock{MaterialID: 1, MaterialName: "灯珠", Type: model.StockTypeIn, Quantity: 10, CreatedAt: now})
	model.DB.Create(&model.MaterialStock{MaterialID: 1, MaterialName: "灯珠", Type: model.StockTypeUse, Quantity: -3, CreatedAt: now})
	model.DB.Create(&model.AIPrediction{DeviceHwID: 1001, RiskLevel: "high", HealthScore: 55, Intersection: "人民路口", PredictType: "x", BatchID: "B2026081501"})
	model.DB.Create(&model.AIPrediction{DeviceHwID: 1002, RiskLevel: "low", HealthScore: 90, Intersection: "建设路口", PredictType: "x", BatchID: "B2026081501"})
}

func TestBuildDailySnapshot_Cov(t *testing.T) {
	reportSeed(t)
	s, err := BuildDailySnapshot()
	if err != nil {
		t.Fatalf("BuildDailySnapshot err: %v", err)
	}
	if s.Devices.Total != 2 {
		t.Errorf("设备 total 期望 2, got %d", s.Devices.Total)
	}
	if s.Devices.Active != 1 {
		t.Errorf("在线 期望 1")
	}
	if s.Faults.Active != 1 {
		t.Errorf("活跃故障 期望 1")
	}
	if s.WorkOrders.Pending != 1 || s.WorkOrders.Processing != 1 || s.WorkOrders.Completed != 1 {
		t.Errorf("工单分布错误: %+v", s.WorkOrders)
	}
	if s.NewExpenses <= 0 {
		t.Errorf("今日费用应>0")
	}
	if len(s.HighRiskDevices) != 1 {
		t.Errorf("高风险设备 期望 1, got %d", len(s.HighRiskDevices))
	}
	if s.HealthSummary == "" {
		t.Error("应有健康摘要")
	}
}

func TestBuildDailySnapshot_NoPredictions(t *testing.T) {
	model.InitTestDB()
	model.DB.Create(&model.Device{HwID: 1, OnlineStatus: true})
	s, _ := BuildDailySnapshot()
	if s.HealthSummary != "尚无预测批次" {
		t.Errorf("无预测批次时摘要=%q", s.HealthSummary)
	}
}

func TestGenerateDailyReport(t *testing.T) {
	reportSeed(t)
	out, err := GenerateDailyReport(1)
	if err != nil {
		t.Fatalf("GenerateDailyReport err: %v", err)
	}
	if out == "" {
		t.Error("报告输出为空")
	}
	// 已入库
	var cnt int64
	model.DB.Model(&model.AIReport{}).Where("module = ?", "daily").Count(&cnt)
	if cnt != 1 {
		t.Errorf("日报应入库 1 条, got %d", cnt)
	}
}

func TestGenerateModuleReport_Cost(t *testing.T) {
	reportSeed(t)
	out, err := GenerateModuleReport(1, "cost", "week")
	if err != nil {
		t.Fatalf("GenerateModuleReport cost err: %v", err)
	}
	if out == "" {
		t.Error("成本报告为空")
	}
	// 非法模块
	if _, err := GenerateModuleReport(1, "bogus", "day"); err == nil {
		t.Error("非法模块应报错")
	}
}

func TestGenerateModuleReport_OtherModules(t *testing.T) {
	reportSeed(t)
	for _, mod := range []string{"inventory", "fault", "workorder", "device"} {
		out, err := GenerateModuleReport(1, mod, "month")
		if err != nil {
			t.Fatalf("GenerateModuleReport %s err: %v", mod, err)
		}
		if out == "" {
			t.Errorf("%s 报告为空", mod)
		}
	}
}

func TestReportHelpers(t *testing.T) {
	// joinFaults / joinDevices 截断（需匿名结构体匹配函数签名）
	fs := joinFaults([]struct {
		FaultType string
		Count     int64
	}{{"a", 1}, {"b", 2}, {"c", 3}, {"d", 4}})
	if len(fs) == 0 {
		t.Error("joinFaults 输出为空")
	}
	ds := joinDevices([]struct {
		DeviceHwID uint32
		Count      int64
	}{{1, 1}, {2, 2}, {3, 3}, {4, 4}})
	if ds == "" {
		t.Error("joinDevices 输出为空")
	}
	// pct
	if pct(1, 0) != 0 || pct(50, 100) != 50 {
		t.Error("pct 计算错误")
	}
	// periodRange
	if f, _ := periodRange("week"); f == "" {
		t.Error("week periodRange 空")
	}
	if f, _ := periodRange("month"); f == "" {
		t.Error("month periodRange 空")
	}
	if f, _ := periodRange("day"); f == "" {
		t.Error("day periodRange 空")
	}
	// ListReports
	reportSeed(t)
	GenerateDailyReport(1)
	lst := ListReports("daily", 5)
	if len(lst) < 1 {
		t.Error("ListReports(daily) 应返回>=1")
	}
	lst2 := ListReports("", 0)
	if len(lst2) < 1 {
		t.Error("ListReports 全量应返回>=1")
	}
	lst3 := ListReports("daily", 1000)
	if len(lst3) > 100 {
		t.Errorf("ListReports 限流: got %d", len(lst3))
	}
	// buildDailyRuleSummary 直接调用
	s := &DailySnapshot{Date: "2026-08-16"}
	rs := buildDailyRuleSummary(s)
	if rs == "" {
		t.Error("buildDailyRuleSummary 输出为空")
	}
}

func TestBuildDailyRuleSummary_Edge(t *testing.T) {
	s := &DailySnapshot{Date: "2026-08-16", OverdueOrders: 2}
	rs := buildDailyRuleSummary(s)
	if rs == "" {
		t.Error("含超时的摘要为空")
	}
	// 高风险设备分支
	s2 := &DailySnapshot{Date: "2026-08-16", HighRiskDevices: []MaterialMin{{Name: "设备#1"}}}
	rs2 := buildDailyRuleSummary(s2)
	if rs2 == "" {
		t.Error("含高风险设备的摘要为空")
	}
}
