package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

func aiAdvanceEngine(t *testing.T) *gin.Engine {
	t.Helper()
	model.InitTestDB()
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/ai/analyze/inventory", AnalyzeInventoryAPI)
		g.GET("/ai/analyze/cost", AnalyzeCostAPI)
		g.POST("/ai/report/generate", GenerateReportAPI)
		g.GET("/ai/reports", ListReportsAPI)
		g.GET("/ai/advice/fault/:id", SuggestFaultAdviceAPI)
		g.GET("/ai/advice/workorder/:id", SuggestWorkOrderAdviceAPI)
		g.GET("/ai/advices", ListAdvicesAPI)
		g.POST("/ai/advice/device", SuggestDeviceCopilotAPI)
		g.POST("/ai/advice/workorder/create", SuggestWorkOrderCreateAPI)
		g.POST("/ai/advice/purchase", SuggestPurchaseCopilotAPI)
		g.POST("/ai/nl/interact", NLInteractAPI)
		g.POST("/ai/decision/center", DecisionCenterAPI)
		g.POST("/ai/decision/adopt", AdoptDecisionAPI)
		g.GET("/ai/anomaly/stream", AnomalyStreamAPI)
	}
	return r
}

func seedAiAdvance(t *testing.T) (uint, uint) {
	t.Helper()
	op := model.User{Username: "op_adv", PasswordHash: "x", Role: model.RoleOperator}
	model.DB.Create(&op)
	model.DB.Create(&model.Device{HwID: 9001, Intersection: "分析路口", OnlineStatus: true})
	model.DB.Create(&model.Material{Code: "M9", Name: "分析灯珠", Category: "灯泡", Stock: 3, Threshold: 5, UnitPrice: 60, Status: "active"})
	now := time.Now()
	model.DB.Create(&model.MaterialStock{MaterialID: 1, MaterialName: "分析灯珠", Type: model.StockTypeIn, Quantity: 5, CreatedAt: now})
	model.DB.Create(&model.RepairExpense{ExpenseNo: "FE9", Type: "material", Amount: 2000, DeviceHwID: 9001, CreatedAt: now})
	f := model.FaultRecord{DeviceHwID: 9001, ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical",
		Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now}
	model.DB.Create(&f)
	wo := model.WorkOrder{OrderNo: "WOadv9", FaultID: f.ID, DeviceHwID: 9001, Status: model.WorkOrderStatusProcessing}
	model.DB.Create(&wo)
	return f.ID, wo.ID
}

func TestAiAdvance_Analyze(t *testing.T) {
	r := aiAdvanceEngine(t)
	seedAiAdvance(t)
	code, body := doReq(t, r, "GET", "/api/v1/ai/analyze/inventory", "")
	mustOK(t, code, body, "库存分析")
	code, body = doReq(t, r, "GET", "/api/v1/ai/analyze/cost", "")
	mustOK(t, code, body, "成本分析")
	code, body = doReq(t, r, "GET", "/api/v1/ai/analyze/cost?days=30", "")
	mustOK(t, code, body, "成本分析带天数")
}

func TestAiAdvance_ReportGenerate(t *testing.T) {
	r := aiAdvanceEngine(t)
	seedAiAdvance(t)
	// daily
	code, body := doReq(t, r, "POST", "/api/v1/ai/report/generate", `{"module":"daily"}`)
	mustOK(t, code, body, "日报生成")
	// inventory module
	code, body = doReq(t, r, "POST", "/api/v1/ai/report/generate", `{"module":"inventory","period":"week"}`)
	mustOK(t, code, body, "库存报告")
	// 缺 module
	code, _ = doReq(t, r, "POST", "/api/v1/ai/report/generate", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺module应 400, got %d", code)
	}
	// 非法模块
	code, _ = doReq(t, r, "POST", "/api/v1/ai/report/generate", `{"module":"bogus"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法module应 400, got %d", code)
	}
	// 历史列表
	code, body = doReq(t, r, "GET", "/api/v1/ai/reports?module=daily&limit=5", "")
	mustOK(t, code, body, "报告列表")
	code, body = doReq(t, r, "GET", "/api/v1/ai/reports", "")
	mustOK(t, code, body, "报告全量")
}

func TestAiAdvance_Advice(t *testing.T) {
	r := aiAdvanceEngine(t)
	fid, wid := seedAiAdvance(t)
	// 故障建议
	code, body := doReq(t, r, "GET", "/api/v1/ai/advice/fault/"+uid(fid), "")
	mustOK(t, code, body, "故障建议")
	// 不存在故障
	code, _ = doReq(t, r, "GET", "/api/v1/ai/advice/fault/99999", "")
	if code != http.StatusNotFound && code != http.StatusInternalServerError {
		t.Errorf("不存在故障建议应 4xx/500, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "GET", "/api/v1/ai/advice/fault/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
	// 工单 copilot + summary
	code, body = doReq(t, r, "GET", "/api/v1/ai/advice/workorder/"+uid(wid), "")
	mustOK(t, code, body, "工单copilot")
	code, body = doReq(t, r, "GET", "/api/v1/ai/advice/workorder/"+uid(wid)+"?stage=summary", "")
	mustOK(t, code, body, "工单summary")
	// 设备 copilot
	code, body = doReq(t, r, "POST", "/api/v1/ai/advice/device", `{"device_hw_id":9001}`)
	mustOK(t, code, body, "设备copilot")
	// 建单 copilot
	code, body = doReq(t, r, "POST", "/api/v1/ai/advice/workorder/create", `{"fault_id":`+uid(fid)+`}`)
	mustOK(t, code, body, "建单copilot")
	// 建单 copilot 缺 fault_id
	code, _ = doReq(t, r, "POST", "/api/v1/ai/advice/workorder/create", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺fault_id应 400, got %d", code)
	}
	// 采购 copilot
	code, body = doReq(t, r, "POST", "/api/v1/ai/advice/purchase", `{"items":[{"material_name":"分析灯珠","quantity":5}]}`)
	mustOK(t, code, body, "采购copilot")
	// 建议历史
	code, body = doReq(t, r, "GET", "/api/v1/ai/advices?biz_type=fault&biz_id="+uid(fid)+"&limit=5", "")
	mustOK(t, code, body, "建议历史")
	code, body = doReq(t, r, "GET", "/api/v1/ai/advices", "")
	mustOK(t, code, body, "建议历史全量")
}

func TestAiAdvance_NLAndDecision(t *testing.T) {
	r := aiAdvanceEngine(t)
	seedAiAdvance(t)
	// NL 查询
	code, body := doReq(t, r, "POST", "/api/v1/ai/nl/interact", `{"text":"运维健康评分是多少"}`)
	mustOK(t, code, body, "NL健康")
	// NL 空文本
	code, _ = doReq(t, r, "POST", "/api/v1/ai/nl/interact", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("NL空文本应 400, got %d", code)
	}
	// 决策中心
	code, body = doReq(t, r, "POST", "/api/v1/ai/decision/center", `{}`)
	mustOK(t, code, body, "决策中心")
	// 一键采纳
	code, body = doReq(t, r, "POST", "/api/v1/ai/decision/adopt", `{"category":"备件采购","items":[{"material_name":"分析灯珠","quantity":3}]}`)
	if code != http.StatusOK && code != http.StatusBadRequest {
		t.Errorf("一键采纳 code=%d body=%v", code, body)
	}
	// 不支持类别
	code, _ = doReq(t, r, "POST", "/api/v1/ai/decision/adopt", `{"category":"成本优化","items":[]}`)
	if code != http.StatusBadRequest {
		t.Errorf("不支持类别应 400, got %d", code)
	}
	// 异常流
	code, body = doReq(t, r, "GET", "/api/v1/ai/anomaly/stream?hours=24&limit=20", "")
	mustOK(t, code, body, "异常流")
	code, body = doReq(t, r, "GET", "/api/v1/ai/anomaly/stream?hours=-1", "")
	mustOK(t, code, body, "异常流hours非法回退")
}
