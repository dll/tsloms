package handler

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

func dashboardEngine(t *testing.T) *gin.Engine {
	t.Helper()
	model.InitTestDB()
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/dashboard/fault-type-stats", FaultTypeStats)
		g.GET("/dashboard/work-order-stats", WorkOrderStatusStats)
		g.GET("/dashboard/fault-trend", FaultTrendStats)
		g.GET("/dashboard/device-fault-rank", DeviceFaultRank)
		g.GET("/dashboard/overview", DashboardOverview)
		g.GET("/dashboard/work-order-avg-closure", WorkOrderAvgClosure)
		g.GET("/dashboard/ai-overview", AIDashboardOverview)
	}
	return r
}

func dashboardSeed(t *testing.T) {
	t.Helper()
	now := time.Now()
	model.DB.Create(&model.Device{HwID: 1, Intersection: "甲路口", OnlineStatus: true})
	model.DB.Create(&model.Device{HwID: 2, Intersection: "乙路口", OnlineStatus: false})
	model.DB.Create(&model.FaultRecord{DeviceHwID: 1, FaultType: "lamp_off", FaultLevel: "critical", Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now})
	model.DB.Create(&model.FaultRecord{DeviceHwID: 1, FaultType: "lamp_off", Status: model.FaultStatusResolved, FirstSeen: now, LastSeen: now})
	// 已完成工单（闭环）
	created := now.Add(-48 * time.Hour)
	closed := now.Add(-24 * time.Hour)
	model.DB.Create(&model.WorkOrder{OrderNo: "WOd1", Status: model.WorkOrderStatusPending, CreatedAt: now.Add(-72 * time.Hour)})
	model.DB.Create(&model.WorkOrder{OrderNo: "WOd2", Status: model.WorkOrderStatusProcessing, CreatedAt: now.Add(-72 * time.Hour)})
	model.DB.Create(&model.WorkOrder{OrderNo: "WOd3", Status: model.WorkOrderStatusCompleted, CreatedAt: created, ClosedAt: &closed})
	// AI 预测 + 用量
	model.DB.Create(&model.AIPrediction{DeviceHwID: 1, BatchID: "B2026081601", RiskLevel: "high", HealthScore: 40, PredictType: "x"})
	model.DB.Create(&model.AIPrediction{DeviceHwID: 2, BatchID: "B2026081601", RiskLevel: "low", HealthScore: 90, PredictType: "x"})
	model.DB.Create(&model.AIUsage{UserID: 0, Action: "predict", Tokens: 50})
}

func TestDashboard_FaultStats(t *testing.T) {
	r := dashboardEngine(t)
	dashboardSeed(t)
	code, body := doReq(t, r, "GET", "/api/v1/dashboard/fault-type-stats?days=30", "")
	mustOK(t, code, body, "故障类型统计")
	if body["data"].(map[string]interface{})["days"].(float64) != 30 {
		t.Errorf("days 字段错误")
	}
	// days 非法
	code, _ = doReq(t, r, "GET", "/api/v1/dashboard/fault-type-stats?days=abc", "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "days非法回退")
}

func TestDashboard_WorkOrderStats(t *testing.T) {
	r := dashboardEngine(t)
	dashboardSeed(t)
	code, body := doReq(t, r, "GET", "/api/v1/dashboard/work-order-stats", "")
	mustOK(t, code, body, "工单状态统计")
	if body["data"].(map[string]interface{})["overdue"].(float64) != 2 {
		t.Errorf("overdue 期望 2(pending超24h+proc超48h), got %v", body["data"].(map[string]interface{})["overdue"])
	}
}

func TestDashboard_FaultTrend(t *testing.T) {
	r := dashboardEngine(t)
	dashboardSeed(t)
	for _, q := range []string{"?dimension=day", "?dimension=week", "?dimension=month", "?dimension=bogus&days=7", "?days=abc"} {
		code, body := doReq(t, r, "GET", "/api/v1/dashboard/fault-trend"+q, "")
		mustOK(t, code, body, "故障趋势 "+q)
	}
}

func TestDashboard_DeviceFaultRank(t *testing.T) {
	r := dashboardEngine(t)
	dashboardSeed(t)
	code, body := doReq(t, r, "GET", "/api/v1/dashboard/device-fault-rank?limit=5&days=30", "")
	mustOK(t, code, body, "设备故障排行")
	rank := body["data"].(map[string]interface{})["rank"].([]interface{})
	if len(rank) < 1 {
		t.Errorf("rank 应非空, got %d", len(rank))
	}
	// limit 非法
	code, _ = doReq(t, r, "GET", "/api/v1/dashboard/device-fault-rank?limit=abc", "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "limit非法回退")
}

func TestDashboard_Overview(t *testing.T) {
	r := dashboardEngine(t)
	dashboardSeed(t)
	code, body := doReq(t, r, "GET", "/api/v1/dashboard/overview", "")
	mustOK(t, code, body, "看板总览")
	data := body["data"].(map[string]interface{})
	if data["devices"].(map[string]interface{})["online"].(float64) != 1 {
		t.Errorf("online 期望 1")
	}
	if data["faults"].(map[string]interface{})["active"].(float64) != 1 {
		t.Errorf("active 故障期望 1")
	}
	if data["work_orders"].(map[string]interface{})["overdue"].(float64) != 2 {
		t.Errorf("overdue 工单期望 2")
	}
}

func TestDashboard_AvgClosure(t *testing.T) {
	r := dashboardEngine(t)
	dashboardSeed(t)
	code, body := doReq(t, r, "GET", "/api/v1/dashboard/work-order-avg-closure?days=30", "")
	mustOK(t, code, body, "平均闭环")
	if body["data"].(map[string]interface{})["completed_count"].(float64) != 1 {
		t.Errorf("completed_count 期望 1")
	}
	if body["data"].(map[string]interface{})["avg_hours"].(float64) <= 0 {
		t.Errorf("avg_hours 应>0")
	}
	// 空库
	r2 := dashboardEngine(t)
	code, body = doReq(t, r2, "GET", "/api/v1/dashboard/work-order-avg-closure", "")
	mustOK(t, code, body, "空库闭环")
}

func TestDashboard_AIOverview(t *testing.T) {
	r := dashboardEngine(t)
	dashboardSeed(t)
	code, body := doReq(t, r, "GET", "/api/v1/dashboard/ai-overview", "")
	mustOK(t, code, body, "AI看板")
	data := body["data"].(map[string]interface{})
	if data["risk_distribution"] == nil {
		t.Errorf("应有风险分布")
	}
	if data["config"] == nil {
		t.Errorf("应有配置")
	}

	// 无预测批次
	r2 := dashboardEngine(t)
	code, body = doReq(t, r2, "GET", "/api/v1/dashboard/ai-overview", "")
	mustOK(t, code, body, "AI看板无预测")
	if body["data"].(map[string]interface{})["risk_distribution"] == nil {
		t.Errorf("无预测也应返回 risk_distribution")
	}
}
