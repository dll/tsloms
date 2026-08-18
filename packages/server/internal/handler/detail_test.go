package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// detailEngine 带故障/工单详情路由的引擎
func detailEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.GET("/faults/:id", GetFault)
		api.GET("/work-orders/:id", GetWorkOrder)
	}
	return r
}

// seedDevice 创建设备（hw_id=1）
func seedDevice(t *testing.T) {
	t.Helper()
	if err := model.DB.Create(&model.Device{HwID: "1", Intersection: "测试路口"}).Error; err != nil {
		t.Fatalf("创建设备失败: %v", err)
	}
}

func TestGetWorkOrder_Detail(t *testing.T) {
	r := detailEngine(t)
	seedDevice(t)
	id := seedFaultAndOrder(t)

	w := doJSON(r, "GET", "/api/v1/work-orders/"+itoa(id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			WorkOrder map[string]interface{} `json:"work_order"`
			SLA       map[string]interface{} `json:"sla"`
			Fault     map[string]interface{} `json:"fault"`
			Assignee  string                 `json:"assignee"`
			Timeline  []interface{}          `json:"timeline"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("业务码 = %d", resp.Code)
	}
	if resp.Data.WorkOrder["id"] == nil {
		t.Error("work_order.id 缺失")
	}
	// SLA 必须携带 stage 与 overdue
	if resp.Data.SLA["stage"] == nil {
		t.Error("sla.stage 缺失")
	}
	if _, has := resp.Data.SLA["overdue"]; !has {
		t.Error("sla.overdue 缺失")
	}
	// 关联故障应带出设备
	faultDev, ok := resp.Data.Fault["device"].(map[string]interface{})
	if !ok {
		t.Error("fault.device 应为对象（关联设备信息）")
	} else if faultDev["intersection"] != "测试路口" {
		t.Errorf("fault.device.intersection = %v", faultDev["intersection"])
	}
}

func TestGetWorkOrder_NotFound(t *testing.T) {
	r := detailEngine(t)
	w := doJSON(r, "GET", "/api/v1/work-orders/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("不存在工单状态码 = %d, 期望 404", w.Code)
	}
}

func TestGetFault_DetailWithDeviceAndOrder(t *testing.T) {
	r := detailEngine(t)
	seedDevice(t)
	id := seedFaultAndOrder(t)

	w := doJSON(r, "GET", "/api/v1/faults/"+itoa(id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Fault     map[string]interface{} `json:"fault"`
			Device    map[string]interface{} `json:"device"`
			WorkOrder map[string]interface{} `json:"work_order"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("业务码 = %d", resp.Code)
	}
	if resp.Data.Device["intersection"] != "测试路口" {
		t.Errorf("device.intersection = %v, 期望 测试路口", resp.Data.Device["intersection"])
	}
	// 关联工单应带出 order_no
	if resp.Data.WorkOrder["order_no"] == nil {
		t.Error("work_order.order_no 缺失（故障应关联工单摘要）")
	}
}

func TestGetFault_NotFound(t *testing.T) {
	r := detailEngine(t)
	w := doJSON(r, "GET", "/api/v1/faults/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("不存在故障状态码 = %d, 期望 404", w.Code)
	}
}
