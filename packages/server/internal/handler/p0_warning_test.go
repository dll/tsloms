package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// P0-3 预警管理：记录列表/忽略/批量忽略/转工单/导出 + 预警配置(忽略规则)CRUD/自动忽略
// ============================================================================

func p0WarningEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.GET("/warnings", ListWarnings)
		api.GET("/warnings/export", ExportWarnings)
		api.GET("/warnings/:id", GetWarning)
		api.POST("/warnings/:id/ignore", IgnoreWarning)
		api.POST("/warnings/batch-ignore", BatchIgnoreWarnings)
		api.POST("/warnings/:id/to-workorder", WarningToWorkOrder)
		api.POST("/warnings/auto-ignore", AutoIgnoreWarnings)
		api.GET("/warning-rules", ListWarningRules)
		api.POST("/warning-rules", CreateWarningRule)
		api.PUT("/warning-rules/:id", UpdateWarningRule)
		api.DELETE("/warning-rules/:id", DeleteWarningRule)
	}
	return r
}

func seedWarning(hw uint32, code int, level string, source string) model.Warning {
	now := time.Now()
	w := model.Warning{
		DeviceHwID:   hw,
		WarningCode:  code,
		WarningLabel: "测试告警",
		Level:        level,
		Source:       source,
		DealState:    model.WarningDealUnhandled,
		Status:       model.WarningUntransferred,
		OccurredAt:   now,
	}
	model.DB.Create(&w)
	return w
}

func TestWarnings_ListAndFilter(t *testing.T) {
	r := p0WarningEngine(t)
	seedWarning(1, -1, model.WarningLevelCritical, model.WarningSourceFault)
	seedWarning(2, -5, model.WarningLevelInfo, model.WarningSourceMQTT)
	seedWarning(3, -1, model.WarningLevelWarning, model.WarningSourceSelfCheck)

	_, body := doReq(t, r, "GET", "/api/v1/warnings", "")
	if body["code"].(float64) != 0 {
		t.Fatalf("列表失败: %v", body)
	}
	if body["data"].(map[string]interface{})["total"].(float64) != 3 {
		t.Errorf("total 期望 3, 实际 %v", body["data"].(map[string]interface{})["total"])
	}

	// 按级别过滤
	_, body2 := doReq(t, r, "GET", "/api/v1/warnings?level=info", "")
	if body2["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("level=info 过滤 total 期望 1, 实际 %v", body2["data"].(map[string]interface{})["total"])
	}

	// 按告警码过滤
	_, body3 := doReq(t, r, "GET", "/api/v1/warnings?warning_code=-5", "")
	if body3["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("warning_code 过滤 total 期望 1, 实际 %v", body3["data"].(map[string]interface{})["total"])
	}
}

func TestWarningDetail_IncludeContext(t *testing.T) {
	r := p0WarningEngine(t)
	crossing := model.Crossing{Name: "路口甲", RoadName: "长江中路"}
	model.DB.Create(&crossing)
	w := model.Warning{DeviceHwID: 9, CrossingID: &crossing.ID, WarningCode: -1, Level: model.WarningLevelCritical, Status: model.WarningUntransferred, OccurredAt: time.Now()}
	model.DB.Create(&w)

	_, body := doReq(t, r, "GET", "/api/v1/warnings/"+strconv.FormatUint(uint64(w.ID), 10), "")
	if body["code"].(float64) != 0 {
		t.Fatalf("详情失败: %v", body)
	}
	wd := body["data"].(map[string]interface{})["warning"].(map[string]interface{})
	if wd["crossing_name"] != "路口甲" {
		t.Errorf("详情应带路口名, 实际 %v", wd["crossing_name"])
	}
	if wd["road_name"] != "长江中路" {
		t.Errorf("详情应带道路名, 实际 %v", wd["road_name"])
	}
}

func TestWarning_IgnoreAndBatchIgnore(t *testing.T) {
	r := p0WarningEngine(t)
	w1 := seedWarning(1, -1, model.WarningLevelCritical, model.WarningSourceFault)
	w2 := seedWarning(2, -2, model.WarningLevelWarning, model.WarningSourceMQTT)
	w3 := seedWarning(3, -3, model.WarningLevelInfo, model.WarningSourceFault)

	// 单条忽略
	_, body := doReq(t, r, "POST", "/api/v1/warnings/"+strconv.FormatUint(uint64(w1.ID), 10)+"/ignore", `{"reason":"人工忽略"}`)
	if body["code"].(float64) != 0 {
		t.Fatalf("忽略失败: %v", body)
	}
	var w1r model.Warning
	model.DB.First(&w1r, w1.ID)
	if w1r.DealState != model.WarningDealIgnored {
		t.Errorf("w1 处理后状态 = %s, 期望 ignored", w1r.DealState)
	}

	// 批量忽略 w2,w3
	_, body2 := doReq(t, r, "POST", "/api/v1/warnings/batch-ignore", `{"ids":[`+
		strconv.FormatUint(uint64(w2.ID), 10)+`,`+strconv.FormatUint(uint64(w3.ID), 10)+`]}`)
	if body2["code"].(float64) != 0 {
		t.Fatalf("批量忽略失败: %v", body2)
	}
	if body2["data"].(map[string]interface{})["affected"].(float64) != 2 {
		t.Errorf("批量忽略应影响 2 条, 实际 %v", body2["data"].(map[string]interface{})["affected"])
	}
}

func TestWarning_ToWorkOrder(t *testing.T) {
	r := p0WarningEngine(t)
	// 无来源故障的预警 → 转工单（占位独立工单）
	w := seedWarning(5, -8, model.WarningLevelCritical, model.WarningSourceSelfCheck)

	_, body := doReq(t, r, "POST", "/api/v1/warnings/"+strconv.FormatUint(uint64(w.ID), 10)+"/to-workorder", `{"remark":"紧急转单"}`)
	if body["code"].(float64) != 0 {
		t.Fatalf("转工单失败: %v", body)
	}
	wd := body["data"].(map[string]interface{})["warning"].(map[string]interface{})
	if wd["status"] != model.WarningTransferred {
		t.Errorf("预警转单后 status = %v, 期望 transferred", wd["status"])
	}
	woID := wd["work_order_id"]
	if woID == nil {
		t.Fatalf("转单后应有 work_order_id")
	}
	if body["data"].(map[string]interface{})["work_order"].(map[string]interface{})["order_no"] == "" {
		t.Errorf("应返回工单编号")
	}

	// 重复转单应被拒绝
	_, body2 := doReq(t, r, "POST", "/api/v1/warnings/"+strconv.FormatUint(uint64(w.ID), 10)+"/to-workorder", `{}`)
	if body2["code"].(float64) == 0 {
		t.Errorf("重复转单应被拒绝")
	}
}

func TestWarning_ExportCSV(t *testing.T) {
	// 注意：ExportWarnings 直接写 CSV 到 ResponseWriter，不能走 doReq 的 JSON 反序列化。
	// 单独验证：至少不 panic 且 Content-Type 正确。
	r := p0WarningEngine(t)
	seedWarning(1, -1, model.WarningLevelCritical, model.WarningSourceFault)
	req := newReq("GET", "/api/v1/warnings/export", "")
	wrec := newRecorder()
	r.ServeHTTP(wrec, req)
	if wrec.Code != http.StatusOK {
		t.Errorf("导出应 200, 实际 %d", wrec.Code)
	}
	if ct := wrec.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("CSV Content-Type = %s", ct)
	}
}

func TestWarningRule_CRUD_AndAutoIgnore(t *testing.T) {
	r := p0WarningEngine(t)
	crossing := model.Crossing{Name: "路口X"}
	model.DB.Create(&crossing)

	// 新增规则：忽略路口X的 critical 告警码 -1
	_, body := doReq(t, r, "POST", "/api/v1/warning-rules", `{"name":"忽略路口X严重告警","crossing_id":`+
		strconv.FormatUint(uint64(crossing.ID), 10)+`,"warning_code":-1,"level":"critical","effective_type":"permanent","action":"ignore","enabled":true}`)
	if body["code"].(float64) != 0 {
		t.Fatalf("新增规则失败: %v", body)
	}
	ruleID := uint(body["data"].(map[string]interface{})["rule"].(map[string]interface{})["id"].(float64))

	// 列表应包含
	_, body2 := doReq(t, r, "GET", "/api/v1/warning-rules", "")
	if body2["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("规则列表 total 期望 1, 实际 %v", body2["data"].(map[string]interface{})["total"])
	}

	// 编辑规则名称
	_, body3 := doReq(t, r, "PUT", "/api/v1/warning-rules/"+strconv.FormatUint(uint64(ruleID), 10), `{"name":"改名后的规则"}`)
	if body3["code"].(float64) != 0 {
		t.Fatalf("编辑规则失败: %v", body3)
	}

	// 自动忽略：落入规则的路口X critical -1 未处理预警被忽略；其余不受影响
	wHit := seedWarningBig(crossing.ID, 10, -1, model.WarningLevelCritical)
	wOther := seedWarning(99, -1, model.WarningLevelCritical, model.WarningSourceFault) // 无路口

	_, body4 := doReq(t, r, "POST", "/api/v1/warnings/auto-ignore", "")
	if body4["code"].(float64) != 0 {
		t.Fatalf("自动忽略失败: %v", body4)
	}
	if body4["data"].(map[string]interface{})["affected"].(float64) != 1 {
		t.Errorf("自动忽略应只影响 1 条命中规则预警, 实际 %v", body4["data"].(map[string]interface{})["affected"])
	}
	var wHitR, wOtherR model.Warning
	model.DB.First(&wHitR, wHit.ID)
	model.DB.First(&wOtherR, wOther.ID)
	if wHitR.DealState != model.WarningDealIgnored {
		t.Errorf("命中规则预警应被忽略, 实际 %s", wHitR.DealState)
	}
	if wOtherR.DealState != model.WarningDealUnhandled {
		t.Errorf("未命中规则预警不应被忽略, 实际 %s", wOtherR.DealState)
	}

	// 删除规则
	_, body5 := doReq(t, r, "DELETE", "/api/v1/warning-rules/"+strconv.FormatUint(uint64(ruleID), 10), "")
	if body5["code"].(float64) != 0 {
		t.Fatalf("删除规则失败: %v", body5)
	}
}

// seedWarningBig 构造带路口+设备的预警
func seedWarningBig(crossingID uint, hw uint32, code int, level string) model.Warning {
	now := time.Now()
	w := model.Warning{
		DeviceHwID:   hw,
		CrossingID:   &crossingID,
		WarningCode:  code,
		WarningLabel: "测试",
		Level:        level,
		Source:       model.WarningSourceMQTT,
		DealState:    model.WarningDealUnhandled,
		Status:       model.WarningUntransferred,
		OccurredAt:   now,
	}
	model.DB.Create(&w)
	return w
}

// newReq 构造 httptest 请求（body 可为 JSON 字符串）
func newReq(method, path, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r, _ = http.NewRequest(method, path, nil)
	} else {
		r, _ = http.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func newRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}
