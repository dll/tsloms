package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// newDashboardEngine 构造看板路由（含 AI 看板）
func newDashboardEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	setCtx := func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_role", model.RoleAdmin)
		c.Set("username", "admin")
		c.Next()
	}
	g := r.Group("/dashboard")
	g.Use(setCtx)
	{
		g.GET("/overview", DashboardOverview)
		g.GET("/ai-overview", AIDashboardOverview)
	}
	return r
}

// seedAIDashboardData 造 AI 看板数据：一个设备 + 一批预测 + 若干用量记录
func seedAIDashboardData(t *testing.T) {
	t.Helper()
	model.DB.Create(&model.Device{ID: 1, HwID: "1001"})

	now := time.Now()
	batch := "202608150600"
	preds := []model.AIPrediction{
		{DeviceHwID: "1001", Intersection: "A路口", BatchID: batch, HealthScore: 30, RiskLevel: "critical", PredictType: "lamp_off", RemainDays: 3},
		{DeviceHwID: "1002", Intersection: "B路口", BatchID: batch, HealthScore: 55, RiskLevel: "high", PredictType: "power_loss", RemainDays: 15},
		{DeviceHwID: "1003", Intersection: "C路口", BatchID: batch, HealthScore: 80, RiskLevel: "medium", PredictType: "dim", RemainDays: 60},
	}
	for i := range preds {
		preds[i].CreatedAt = now
	}
	model.DB.Create(&preds)

	usages := []model.AIUsage{
		{UserID: 1, Action: "predict", Model: "glm-4-flash", TokensIn: 100, TokensOut: 50, Tokens: 150, OK: true, CreatedAt: now},
		{UserID: 1, Action: "diagnose", Model: "glm-4v", TokensIn: 200, TokensOut: 80, Tokens: 280, OK: true, CreatedAt: now},
	}
	model.DB.Create(&usages)
}

func TestAIDashboardOverview(t *testing.T) {
	r := newDashboardEngine(t)
	seedAIDashboardData(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/dashboard/ai-overview", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("AI看板状态码=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Today struct {
				Tokens int64 `json:"tokens"`
				Calls  int   `json:"calls"`
			} `json:"today"`
			RiskDistribution map[string]interface{} `json:"risk_distribution"`
			HighRiskDevices  []gin.H                `json:"high_risk_devices"`
			ActionSummary    map[string]interface{} `json:"action_summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if resp.Data.Today.Tokens != 430 {
		t.Errorf("今日token = %d, 期望430", resp.Data.Today.Tokens)
	}
	if resp.Data.Today.Calls != 2 {
		t.Errorf("今日调用 = %d, 期望2", resp.Data.Today.Calls)
	}
	if resp.Data.RiskDistribution["total"] != float64(3) {
		t.Errorf("风险分布total = %v, 期望3", resp.Data.RiskDistribution["total"])
	}
	if resp.Data.RiskDistribution["high"] != float64(1) || resp.Data.RiskDistribution["critical"] != float64(1) {
		t.Errorf("风险分布 high/critical 异常: %v", resp.Data.RiskDistribution)
	}
	// 高/极高风险设备应包含 1001(critical) 和 1002(high)
	if len(resp.Data.HighRiskDevices) != 2 {
		t.Errorf("高风险设备数 = %d, 期望2", len(resp.Data.HighRiskDevices))
	}
	if resp.Data.ActionSummary["predict"] != float64(1) || resp.Data.ActionSummary["diagnose"] != float64(1) {
		t.Errorf("动作汇总异常: %v", resp.Data.ActionSummary)
	}
}
