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

// setupHandlerEngine 构造带主要查询路由的测试引擎
func setupHandlerEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.GET("/devices", ListDevices)
		api.GET("/devices/stats", DeviceStats)
		api.GET("/faults", ListFaults)
		api.GET("/dashboard/fault-type-stats", FaultTypeStats)
		api.GET("/dashboard/work-order-stats", WorkOrderStatusStats)
		api.GET("/user/info", GetUserInfo)
	}
	return r
}

func getJSON(t *testing.T, r *gin.Engine, path string) (int, map[string]interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

func TestListDevices_PaginationAndFilter(t *testing.T) {
	r := setupHandlerEngine(t)
	model.DB.Create(&model.Device{HwID: 1, OnlineStatus: true, Intersection: "路口A"})
	model.DB.Create(&model.Device{HwID: 2, OnlineStatus: false})
	model.DB.Create(&model.Device{HwID: 3, OnlineStatus: true})

	// 全量
	code, body := getJSON(t, r, "/api/v1/devices")
	if code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("code=%d body=%v", code, body)
	}
	data := body["data"].(map[string]interface{})
	if data["total"].(float64) != 3 {
		t.Errorf("total = %v, 期望 3", data["total"])
	}

	// 筛选在线
	_, body2 := getJSON(t, r, "/api/v1/devices?online_status=true")
	data2 := body2["data"].(map[string]interface{})
	if data2["total"].(float64) != 2 {
		t.Errorf("在线筛选中 total = %v, 期望 2", data2["total"])
	}

	// 分页
	_, body3 := getJSON(t, r, "/api/v1/devices?page=2&page_size=2")
	data3 := body3["data"].(map[string]interface{})
	list3 := data3["list"].([]interface{})
	if len(list3) != 1 {
		t.Errorf("第2页应返回 1 条, 实际 %d", len(list3))
	}
}

func TestDeviceStats(t *testing.T) {
	r := setupHandlerEngine(t)
	model.DB.Create(&model.Device{HwID: 1, OnlineStatus: true})
	model.DB.Create(&model.Device{HwID: 2, OnlineStatus: false})
	model.DB.Create(&model.Device{HwID: 3, OnlineStatus: true})

	_, body := getJSON(t, r, "/api/v1/devices/stats")
	data := body["data"].(map[string]interface{})
	if data["online"].(float64) != 2 || data["offline"].(float64) != 1 {
		t.Errorf("stats = %v, 期望 online=2 offline=1", data)
	}
}

func TestListFaults_DateFilterBothParamNames(t *testing.T) {
	r := setupHandlerEngine(t)
	now := time.Now()
	model.DB.Create(&model.FaultRecord{DeviceHwID: 1, ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical", Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now})

	// start_date（前端用名）
	_, body := getJSON(t, r, "/api/v1/faults?start_date=2026-08-01&end_date=2026-08-30")
	data := body["data"].(map[string]interface{})
	if data["total"].(float64) != 1 {
		t.Errorf("start_date 筛选 total = %v, 期望 1（兼容前端参数名）", data["total"])
	}

	// start_time（后端用名）
	_, body2 := getJSON(t, r, "/api/v1/faults?start_time=2026-08-01&end_time=2026-08-30")
	data2 := body2["data"].(map[string]interface{})
	if data2["total"].(float64) != 1 {
		t.Errorf("start_time 筛选 total = %v, 期望 1", data2["total"])
	}
}

func TestFaultTypeStats_GroupsByType(t *testing.T) {
	r := setupHandlerEngine(t)
	now := time.Now()
	model.DB.Create(&model.FaultRecord{DeviceHwID: 1, ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical", Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now})
	model.DB.Create(&model.FaultRecord{DeviceHwID: 2, ErrCode: -4, FaultType: "abnormal_on", FaultLevel: "critical", Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now})
	model.DB.Create(&model.FaultRecord{DeviceHwID: 3, ErrCode: -8, FaultType: "timeout", FaultLevel: "normal", Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now})

	code, body := getJSON(t, r, "/api/v1/dashboard/fault-type-stats")
	if code != http.StatusOK {
		t.Fatalf("code=%d", code)
	}
	stats := body["data"].(map[string]interface{})["stats"].([]interface{})
	if len(stats) != 3 {
		t.Errorf("应聚合 3 类故障, 实际 %d", len(stats))
	}
}

func TestWorkOrderStatusStats(t *testing.T) {
	r := setupHandlerEngine(t)
	now := time.Now()
	model.DB.Create(&model.WorkOrder{OrderNo: "WO1", Status: "pending", CreatedAt: now})
	model.DB.Create(&model.WorkOrder{OrderNo: "WO2", Status: "completed", CreatedAt: now})

	_, body := getJSON(t, r, "/api/v1/dashboard/work-order-stats")
	stats := body["data"].(map[string]interface{})["stats"].([]interface{})
	var pendingCount float64
	for _, s := range stats {
		m := s.(map[string]interface{})
		if m["status"] == "pending" {
			pendingCount = m["count"].(float64)
		}
	}
	if pendingCount != 1 {
		t.Errorf("pending 工单数 = %v, 期望 1", pendingCount)
	}
}

func TestGetUserInfo_NoUserID(t *testing.T) {
	r := setupHandlerEngine(t)
	// 未注入 user_id（模拟未登录），First 失败应返回 404
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/user/info", nil))
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("无 user_id 时状态码 = %d, 期望 4xx", w.Code)
	}
}
