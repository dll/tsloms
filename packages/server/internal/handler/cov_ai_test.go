package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

func aiHandlerEngine(t *testing.T) *gin.Engine {
	t.Helper()
	model.InitTestDB()
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/ai/config", GetAIConfig)
		g.PUT("/ai/config", UpdateAIConfig)
		g.GET("/ai/usage", MyAIUsage)
		g.POST("/ai/usage/reset", ResetAIUsage)
		g.GET("/ai/usage/logs", AIUsagePage)
		g.POST("/ai/predict/run", RunPrediction)
		g.GET("/ai/predict/by-intersection", RunPredictionByIntersection)
		g.GET("/ai/predict", AIPredictions)
		g.POST("/ai/predict/:id/enhance", EnhancePredictionPlan)
		g.POST("/ai/diagnose/:id", DiagnoseFeedbackAPI)
		g.GET("/ai/lifecycle/:hwid", BuildLifecycleAPI)
	}
	return r
}

func TestAiConfig_ReadUpdate(t *testing.T) {
	r := aiHandlerEngine(t)
	// 读（默认配置，key 为空 → has_key=false）
	code, body := doReq(t, r, "GET", "/api/v1/ai/config", "")
	mustOK(t, code, body, "读AI配置")
	// 更新
	code, _ = doReq(t, r, "PUT", "/api/v1/ai/config", `{"provider":"zhipu","text_model":"glm-4","api_key":"sk-test1234567890","enabled":true,"day_token_limit":10000,"day_call_limit":100}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "更新AI配置")
	// 再次读，key 已脱敏
	code, body = doReq(t, r, "GET", "/api/v1/ai/config", "")
	mustOK(t, code, body, "读AI配置2")
	if body["data"].(map[string]interface{})["has_key"] != true {
		t.Errorf("更新后 has_key 应为 true")
	}
	if body["data"].(map[string]interface{})["api_key_masked"] == "sk-test1234567890" {
		t.Errorf("key 应脱敏")
	}
	// 缺参数(body 非法) → 400
	code, _ = doReq(t, r, "PUT", "/api/v1/ai/config", `not-json`)
	if code != http.StatusBadRequest {
		t.Errorf("非法body应 400, got %d", code)
	}
}

func TestAiUsage(t *testing.T) {
	r := aiHandlerEngine(t)
	// MyAIUsage
	code, body := doReq(t, r, "GET", "/api/v1/ai/usage", "")
	mustOK(t, code, body, "我的额度")
	if body["data"].(map[string]interface{})["enabled"] == nil {
		t.Errorf("应有 enabled 字段")
	}
	// 写入一条用量
	model.DB.Create(&model.AIUsage{UserID: 1, Action: "predict", Model: "glm", Tokens: 100, OK: true})
	// Reset 今日额度
	code, _ = doReq(t, r, "POST", "/api/v1/ai/usage/reset", "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "重置额度")
	// 额度流水
	code, body = doReq(t, r, "GET", "/api/v1/ai/usage/logs", "")
	mustOK(t, code, body, "额度流水")
	code, body = doReq(t, r, "GET", "/api/v1/ai/usage/logs?username=adm", "")
	mustOK(t, code, body, "额度流水按用户")
}

func TestAiPrediction(t *testing.T) {
	r := aiHandlerEngine(t)
	model.DB.Create(&model.Device{HwID: "55", Intersection: "预测路口", OnlineStatus: true, Lat: &lat, Lng: &lng})
	// 全量预测
	code, body := doReq(t, r, "POST", "/api/v1/ai/predict/run", "")
	mustOK(t, code, body, "全量预测")
	if body["data"].(map[string]interface{})["count"].(float64) != 1 {
		t.Errorf("预测 count 期望 1")
	}
	// 按路口聚合
	code, body = doReq(t, r, "GET", "/api/v1/ai/predict/by-intersection", "")
	mustOK(t, code, body, "路口聚合")
	// 历史预测（无 batch_id 全量；有 batch）
	code, body = doReq(t, r, "GET", "/api/v1/ai/predict", "")
	mustOK(t, code, body, "历史预测")
	// 无设备 → 空
	r2 := aiHandlerEngine(t)
	code, body = doReq(t, r2, "POST", "/api/v1/ai/predict/run", "")
	mustOK(t, code, body, "空预测")
}

func TestAiPrediction_Enhance(t *testing.T) {
	r := aiHandlerEngine(t)
	model.DB.Create(&model.AIPrediction{DeviceHwID: "55", RiskLevel: "high", HealthScore: 60, PredictType: "x", RemainDays: 5, Factors: "f"})
	var p model.AIPrediction
	model.DB.First(&p)
	// LLM 无 key → badRequest(400) 或成功
	code, _ := doReq(t, r, "POST", "/api/v1/ai/predict/"+uid(p.ID)+"/enhance", "")
	if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusInternalServerError {
		t.Errorf("enhance code=%d", code)
	}
	// 不存在预测
	code, _ = doReq(t, r, "POST", "/api/v1/ai/predict/99999/enhance", "")
	if code != http.StatusNotFound && code != http.StatusInternalServerError {
		t.Errorf("不存在预测 enhance code=%d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "POST", "/api/v1/ai/predict/abc/enhance", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法ID enhance code=%d", code)
	}
}

func TestAiDiagnoseAndLifecycle(t *testing.T) {
	r := aiHandlerEngine(t)
	model.DB.Create(&model.Device{HwID: "66", Intersection: "诊断路口"})
	// 反馈诊断
	model.DB.Create(&model.Feedback{DeviceHwID: &hw66, Title: "闪烁", Content: "诊断", Status: "open"})
	var fb model.Feedback
	model.DB.First(&fb)
	code, _ := doReq(t, r, "POST", "/api/v1/ai/diagnose/"+uid(fb.ID), "")
	if code != http.StatusOK && code != http.StatusInternalServerError {
		t.Errorf("诊断 code=%d", code)
	}
	// 反馈不存在
	code, _ = doReq(t, r, "POST", "/api/v1/ai/diagnose/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("诊断不存在反馈 code=%d", code)
	}
	// 生命周期
	code, body := doReq(t, r, "GET", "/api/v1/ai/lifecycle/66", "")
	mustOK(t, code, body, "生命周期")
	// 设备不存在
	code, _ = doReq(t, r, "GET", "/api/v1/ai/lifecycle/999", "")
	if code != http.StatusNotFound {
		t.Errorf("生命周期不存在设备 code=%d", code)
	}
	// 非数字 hwid（现在 hwid 为 uuid 字符串，直接按字符串查；不存在设备 → 404）
	code, _ = doReq(t, r, "GET", "/api/v1/ai/lifecycle/abc", "")
	if code != http.StatusNotFound {
		t.Errorf("生命周期非法hwid code=%d", code)
	}
}

func TestAiHelpers(t *testing.T) {
	// maskKey
	if maskKey("short") != "****" {
		t.Error("maskKey 短 key 应 ****")
	}
	if maskKey("1234567890") != "1234...7890" {
		t.Errorf("maskKey=%q", maskKey("1234567890"))
	}
	// userIDFromCtx
	r := gin.New()
	_ = r
	c := &gin.Context{}
	c.Set("user_id", uint(7))
	if userIDFromCtx(c) != 7 {
		t.Error("userIDFromCtx 应 7")
	}
	c2 := &gin.Context{}
	if userIDFromCtx(c2) != 0 {
		t.Error("无 user_id 应 0")
	}
	// mediaRootDir 默认
	if mediaRootDir() != "" {
		t.Error("mediaRootDir 默认应空")
	}
	// resolveFeedbackImages
	if resolveFeedbackImages("", "") != nil {
		t.Error("空参应 nil")
	}
	if resolveFeedbackImages("/tsloms/media/202601/1.jpg", "") != nil {
		t.Error("空 mediaDir 应 nil")
	}
	res := resolveFeedbackImages("/tsloms/media/202601/1.jpg", "/tmp/mediad")
	if len(res) != 1 {
		t.Errorf("正常 URL 应解析 1 个, got %d", len(res))
	}
	// 路径穿越
	if resolveFeedbackImages("/tsloms/media/../../etc/passwd", "/tmp/mediad") != nil {
		t.Error("含 .. 应拒绝")
	}
	if resolveFeedbackImages("no-media-prefix.jpg", "/tmp/mediad") != nil {
		t.Error("无 /media/ 前缀应拒绝")
	}
}

var lat = float64(31.2)
var lng = float64(121.5)
var hw66 = "66"
