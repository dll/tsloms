package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// newGinEngine 构造带路由的测试引擎（使用内存 SQLite）
func newGinEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.POST("/work-orders", CreateWorkOrder)
		api.PUT("/work-orders/:id/status", UpdateWorkOrderStatus)
	}
	return r
}

// seedFault 创建一个故障记录和关联工单，返回故障 ID
func seedFaultAndOrder(t *testing.T) uint {
	t.Helper()
	fault := model.FaultRecord{
		DeviceHwID: 1, ErrCode: -1, FaultType: "lamp_off",
		FaultLevel: "critical", Status: "active",
	}
	if err := model.DB.Create(&fault).Error; err != nil {
		t.Fatalf("创建故障失败: %v", err)
	}
	wo := model.WorkOrder{
		OrderNo: model.NextOrderNo(model.DB), FaultID: fault.ID,
		DeviceHwID: 1, Status: model.WorkOrderStatusPending,
	}
	if err := model.DB.Create(&wo).Error; err != nil {
		t.Fatalf("创建工单失败: %v", err)
	}
	return wo.ID
}

// doJSON 执行 JSON 请求
func doJSON(r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestWorkOrderStatus_PendingToProcessing(t *testing.T) {
	r := newGinEngine(t)
	id := seedFaultAndOrder(t)

	w := doJSON(r, "PUT", "/api/v1/work-orders/"+itoa(id)+"/status",
		map[string]interface{}{"status": "processing"})
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200; body=%s", w.Code, w.Body.String())
	}

	var wo model.WorkOrder
	model.DB.First(&wo, id)
	if wo.Status != model.WorkOrderStatusProcessing {
		t.Errorf("状态 = %s, 期望 processing", wo.Status)
	}
}

func TestWorkOrderStatus_RejectedPreserved(t *testing.T) {
	r := newGinEngine(t)
	id := seedFaultAndOrder(t)

	// 先置为处理中，再驳回
	doJSON(r, "PUT", "/api/v1/work-orders/"+itoa(id)+"/status",
		map[string]interface{}{"status": "processing"})
	w := doJSON(r, "PUT", "/api/v1/work-orders/"+itoa(id)+"/status",
		map[string]interface{}{"status": "rejected", "result": "现场无法维修"})
	if w.Code != http.StatusOK {
		t.Fatalf("驳回状态码 = %d, 期望 200; body=%s", w.Code, w.Body.String())
	}

	var wo model.WorkOrder
	model.DB.First(&wo, id)
	// 修复后：rejected 应被保留（不再被改写为 pending）
	if wo.Status != model.WorkOrderStatusRejected {
		t.Errorf("状态 = %s, 期望 rejected（驳回状态应保留）", wo.Status)
	}
}

func TestWorkOrderStatus_CompletedClosesAndResolvesFault(t *testing.T) {
	r := newGinEngine(t)
	id := seedFaultAndOrder(t)

	// 获取关联故障 ID
	var wo model.WorkOrder
	model.DB.First(&wo, id)
	faultID := wo.FaultID

	w := doJSON(r, "PUT", "/api/v1/work-orders/"+itoa(id)+"/status",
		map[string]interface{}{"status": "completed", "result": "已更换灯珠"})
	if w.Code != http.StatusOK {
		t.Fatalf("完成状态码 = %d; body=%s", w.Code, w.Body.String())
	}

	model.DB.First(&wo, id)
	if wo.Status != model.WorkOrderStatusCompleted {
		t.Errorf("状态 = %s, 期望 completed", wo.Status)
	}
	if wo.ClosedAt == nil {
		t.Error("ClosedAt 应被填充")
	}

	var fault model.FaultRecord
	model.DB.First(&fault, faultID)
	if fault.Status != "resolved" {
		t.Errorf("关联故障状态 = %s, 期望 resolved", fault.Status)
	}
}

func TestWorkOrderStatus_InvalidStatusRejected(t *testing.T) {
	r := newGinEngine(t)
	id := seedFaultAndOrder(t)

	w := doJSON(r, "PUT", "/api/v1/work-orders/"+itoa(id)+"/status",
		map[string]interface{}{"status": "invalid_status"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("非法状态码 = %d, 期望 400", w.Code)
	}
}

func TestWorkOrderStatus_NotFound(t *testing.T) {
	r := newGinEngine(t)
	w := doJSON(r, "PUT", "/api/v1/work-orders/99999/status",
		map[string]interface{}{"status": "processing"})
	if w.Code != http.StatusNotFound {
		t.Errorf("不存在工单状态码 = %d, 期望 404", w.Code)
	}
}

// itoa 简易整数转字符串
func itoa(v uint) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
