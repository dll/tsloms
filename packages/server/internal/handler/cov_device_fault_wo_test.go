package handler

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
	"golang.org/x/crypto/bcrypt"
)

// ==================== 设备 Device ====================

func seedOperator(u string) model.User {
	pwd, _ := bcrypt.GenerateFromPassword([]byte("Test@12345"), bcrypt.MinCost)
	us := model.User{Username: u, PasswordHash: string(pwd), Role: model.RoleOperator, Status: ""}
	model.DB.Create(&us)
	return us
}

func TestDevice_CRUD(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/devices", ListDevices)
	rg.GET("/devices/:id", GetDevice)
	rg.PUT("/devices/:id", UpdateDevice)
	rg.POST("/devices", CreateDevice)
	rg.DELETE("/devices/:id", DeleteDevice)
	rg.GET("/devices/stats", DeviceStats)

	// 新增
	code, body := doReq(t, r, "POST", "/api/v1/devices", `{"hw_id":101,"intersection":"主干道","network_code":5,"station_code":2,"lat":31.2,"lng":121.5,"online_status":true}`)
	mustOK(t, code, body, "新增设备")
	did := uint(body["data"].(map[string]interface{})["device"].(map[string]interface{})["id"].(float64))

	// 重复 hw_id → 400
	code, _ = doReq(t, r, "POST", "/api/v1/devices", `{"hw_id":101}`)
	if code != http.StatusBadRequest {
		t.Errorf("重复 hw_id 应 400, got %d", code)
	}
	// 缺 hw_id
	code, _ = doReq(t, r, "POST", "/api/v1/devices", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺 hw_id 应 400, got %d", code)
	}
	// 列表筛选
	model.DB.Create(&model.Device{HwID: 102, Intersection: "支路", OnlineStatus: false})
	model.DB.Create(&model.Device{HwID: 103, Intersection: "干道二路", OnlineStatus: true})
	qs := url.Values{}
	qs.Set("intersection", "干道")
	qs.Set("hw_id", "101")
	code, body = doReq(t, r, "GET", "/api/v1/devices?"+qs.Encode(), "")
	mustOK(t, code, body, "设备列表筛选(路口+hw)")
	if body["data"].(map[string]interface{})["total"].(float64) < 1 {
		t.Errorf("路口+hw 筛选 total 应≥1")
	}
	// 在线筛选（设备 103 在线）
	code, body = doReq(t, r, "GET", "/api/v1/devices?online_status=true", "")
	mustOK(t, code, body, "在线筛选")
	if body["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("在线筛选 total 期望 1, got %v", body["data"].(map[string]interface{})["total"])
	}
	// 详情 + 版本解码
	code, body = doReq(t, r, "GET", "/api/v1/devices/"+uid(did), "")
	mustOK(t, code, body, "设备详情")
	if body["data"].(map[string]interface{})["sw_ver_info"] == nil {
		t.Errorf("应返回 sw_ver_info")
	}
	// 详情非法ID
	code, _ = doReq(t, r, "GET", "/api/v1/devices/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法详情ID应 400, got %d", code)
	}
	// 详情不存在
	code, _ = doReq(t, r, "GET", "/api/v1/devices/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("详情不存在应 404, got %d", code)
	}
	// 更新
	code, _ = doReq(t, r, "PUT", "/api/v1/devices/"+uid(did), `{"intersection":"新路口","installed_at":"2026-01-01","lat":31.5,"lng":121.6,"is_watched":true}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "更新设备")
	// 更新不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/devices/99999", `{"intersection":"x"}`)
	if code != http.StatusNotFound {
		t.Errorf("更新不存在应 404, got %d", code)
	}
	code, _ = doReq(t, r, "PUT", "/api/v1/devices/abc", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法更新ID应 400, got %d", code)
	}
	// 统计 (设备: 101, 102, 103)
	code, body = doReq(t, r, "GET", "/api/v1/devices/stats", "")
	mustOK(t, code, body, "设备统计")
	if body["data"].(map[string]interface{})["total"].(float64) != 3 {
		t.Errorf("stats total 期望 3, got %v", body["data"].(map[string]interface{})["total"])
	}
	// 删除
	code, _ = doReq(t, r, "DELETE", "/api/v1/devices/"+uid(did), "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "删除设备")
	// 重复删除
	code, _ = doReq(t, r, "DELETE", "/api/v1/devices/"+uid(did), "")
	if code != http.StatusNotFound {
		t.Errorf("重复删除应 404, got %d", code)
	}
}

// ==================== 故障 Fault ====================

func seedFault(hw uint32, status string) model.FaultRecord {
	now := time.Now()
	f := model.FaultRecord{
		DeviceHwID: hw, ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical",
		Status: status, FirstSeen: now, LastSeen: now,
	}
	model.DB.Create(&f)
	return f
}

func TestFault_ListFilters(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/faults", ListFaults)
	seedFault(201, model.FaultStatusOccurred)
	seedFault(201, model.FaultStatusConfirmed)
	seedFault(202, model.FaultStatusResolved)

	for _, q := range []string{
		"?hw_id=201", "?status=active", "?status=resolved", "?fault_type=lamp_off",
		"?fault_level=critical", "?start_time=2026-08-01", "?end_time=2026-08-30",
		"?start_date=2026-08-01&end_date=2026-08-30", "?page=1&page_size=10",
	} {
		code, _ := doReq(t, r, "GET", "/api/v1/faults"+q, "")
		if code != http.StatusOK {
			t.Errorf("筛选 %s 失败 code=%d", q, code)
		}
	}
}

func TestFault_GetAndView(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/faults/:id", GetFault)
	op := seedOperator("op_fault")
	f := model.FaultRecord{DeviceHwID: 301, ErrCode: -4, FaultType: "abnormal_on", FaultLevel: "major",
		Status: model.FaultStatusOccurred, FirstSeen: time.Now(), LastSeen: time.Now(),
		OwnerID: &op.ID, RepairerID: &op.ID}
	model.DB.Create(&f)
	// 关联设备
	model.DB.Create(&model.Device{HwID: 301, Intersection: "故障路口"})
	// 详情
	code, body := doReq(t, r, "GET", "/api/v1/faults/"+uid(f.ID), "")
	mustOK(t, code, body, "故障详情")
	fv := body["data"].(map[string]interface{})["fault"].(map[string]interface{})
	if fv["owner_name"] != "op_fault" {
		t.Errorf("owner_name=%v", fv["owner_name"])
	}
	if body["data"].(map[string]interface{})["device"].(map[string]interface{})["intersection"] != "故障路口" {
		t.Errorf("关联设备未带出")
	}
	// 非法ID / 不存在
	code, _ = doReq(t, r, "GET", "/api/v1/faults/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
	code, _ = doReq(t, r, "GET", "/api/v1/faults/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("不存在应 404, got %d", code)
	}
}

func TestFault_UpdateTransitions(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.PUT("/faults/:id", UpdateFault)
	op := seedOperator("op_f2")
	f := seedFault(401, model.FaultStatusOccurred)

	// 无效状态
	code, _ := doReq(t, r, "PUT", "/api/v1/faults/"+uid(f.ID), `{"status":"bogus"}`)
	if code != http.StatusBadRequest {
		t.Errorf("无效状态应 400, got %d", code)
	}
	// 设置负责人 → 自动确认，默认推进 status
	code, body := doReq(t, r, "PUT", "/api/v1/faults/"+uid(f.ID), `{"owner_id":`+uid(op.ID)+`}`)
	mustOK(t, code, body, "设负责人")
	// 显式派单
	code, _ = doReq(t, r, "PUT", "/api/v1/faults/"+uid(f.ID), `{"status":"dispatched","repairer_id":`+uid(op.ID)+`}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "派单")
	// 解决
	code, _ = doReq(t, r, "PUT", "/api/v1/faults/"+uid(f.ID), `{"status":"resolved"}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "解决")
	// 负责人不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/faults/"+uid(f.ID), `{"owner_id":99999}`)
	if code != http.StatusNotFound {
		t.Errorf("负责人不存在应 404, got %d", code)
	}
	// 维修人不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/faults/"+uid(f.ID), `{"repairer_id":99999}`)
	if code != http.StatusNotFound {
		t.Errorf("维修人不存在应 404, got %d", code)
	}
	// 无可更新字段
	code, _ = doReq(t, r, "PUT", "/api/v1/faults/"+uid(f.ID), `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("无字段应 400, got %d", code)
	}
	// 非法ID / 不存在 / 参数错误
	code, _ = doReq(t, r, "PUT", "/api/v1/faults/abc", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400")
	}
	code, _ = doReq(t, r, "PUT", "/api/v1/faults/99999", `{"status":"confirmed"}`)
	if code != http.StatusNotFound {
		t.Errorf("不存在应 404")
	}
}

func TestFault_DeleteProtection(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.DELETE("/faults/:id", DeleteFault)
	f := seedFault(501, model.FaultStatusOccurred)
	// 有关联未完成工单 → 拒绝
	model.DB.Create(&model.WorkOrder{FaultID: f.ID, OrderNo: "WOx", Status: model.WorkOrderStatusPending})
	code, _ := doReq(t, r, "DELETE", "/api/v1/faults/"+uid(f.ID), "")
	if code != http.StatusBadRequest {
		t.Errorf("有未完成工单应 400, got %d", code)
	}
	model.DB.Where("fault_id = ?", f.ID).Delete(&model.WorkOrder{})
	// 正常删除
	code, _ = doReq(t, r, "DELETE", "/api/v1/faults/"+uid(f.ID), "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "删除故障")
	// 非法ID / 不存在
	code, _ = doReq(t, r, "DELETE", "/api/v1/faults/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400")
	}
	code, _ = doReq(t, r, "DELETE", "/api/v1/faults/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("不存在应 404")
	}
}

func TestFault_Dispatch(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.POST("/faults/:id/dispatch", DispatchFault)
	op := seedOperator("op_disp")
	f := seedFault(601, model.FaultStatusConfirmed)
	// 派单
	code, body := doReq(t, r, "POST", "/api/v1/faults/"+uid(f.ID)+"/dispatch", `{"assignee_id":`+uid(op.ID)+`}`)
	mustOK(t, code, body, "派单")
	// 缺 assignee
	code, _ = doReq(t, r, "POST", "/api/v1/faults/"+uid(f.ID)+"/dispatch", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺assignee应 400, got %d", code)
	}
	// 维修人不存在
	code, _ = doReq(t, r, "POST", "/api/v1/faults/"+uid(f.ID)+"/dispatch", `{"assignee_id":99999}`)
	if code != http.StatusNotFound {
		t.Errorf("维修人不存在应 404, got %d", code)
	}
	// viewer 不能派
	vw := model.User{Username: "vw_disp", PasswordHash: "x", Role: model.RoleViewer}
	model.DB.Create(&vw)
	code, _ = doReq(t, r, "POST", "/api/v1/faults/"+uid(f.ID)+"/dispatch", `{"assignee_id":`+uid(vw.ID)+`}`)
	if code != http.StatusBadRequest {
		t.Errorf("viewer 派单应 400, got %d", code)
	}
	// 非法ID / 不存在
	code, _ = doReq(t, r, "POST", "/api/v1/faults/abc/dispatch", `{"assignee_id":1}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400")
	}
	code, _ = doReq(t, r, "POST", "/api/v1/faults/99999/dispatch", `{"assignee_id":1}`)
	if code != http.StatusNotFound {
		t.Errorf("不存在应 404")
	}
}

// ==================== 工单 WorkOrder ====================

func TestWorkOrder_CRUD(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/work-orders", ListWorkOrders)
	rg.GET("/work-orders/:id", GetWorkOrder)
	rg.POST("/work-orders", CreateWorkOrder)
	rg.PUT("/work-orders/:id/status", UpdateWorkOrderStatus)
	rg.PUT("/work-orders/:id/assign", AssignWorkOrder)
	rg.DELETE("/work-orders/:id", DeleteWorkOrder)

	op := seedOperator("op_wo")
	f := seedFault(701, model.FaultStatusOccurred)
	model.DB.Create(&model.Device{HwID: 701, Intersection: "工单路口"})

	// 创建（关联故障 + 指派运维）
	code, body := doReq(t, r, "POST", "/api/v1/work-orders", `{"fault_id":`+uid(f.ID)+`,"device_hw_id":701,"assignee_id":`+uid(op.ID)+`}`)
	mustOK(t, code, body, "创建工单")
	wid := uint(body["data"].(map[string]interface{})["work_order"].(map[string]interface{})["id"].(float64))

	// 列表筛选
	for _, q := range []string{"", "?hw_id=701", "?device_hw_id=701", "?status=pending", "?assignee_id=" + uid(op.ID), "?order_no=WO", "?start_time=2026-08-01"} {
		code, _ := doReq(t, r, "GET", "/api/v1/work-orders"+q, "")
		if code != http.StatusOK {
			t.Errorf("工单列表 %s 失败 code=%d", q, code)
		}
	}
	// 详情
	code, body = doReq(t, r, "GET", "/api/v1/work-orders/"+uid(wid), "")
	mustOK(t, code, body, "工单详情")
	if body["data"].(map[string]interface{})["sla"] == nil {
		t.Errorf("应返回 SLA 信息")
	}
	// 更新状态 → 完成
	code, _ = doReq(t, r, "PUT", "/api/v1/work-orders/"+uid(wid)+"/status", `{"status":"completed","result":"已修复"}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "完成工单")
	// 完成应推进故障为 resolved
	model.DB.First(&f, f.ID)
	if f.Status != model.FaultStatusResolved {
		t.Errorf("完成工单后故障应 resolved, got %s", f.Status)
	}
	// 删除
	code, _ = doReq(t, r, "DELETE", "/api/v1/work-orders/"+uid(wid), "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "删除工单")
}

func TestWorkOrder_Validation(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.POST("/work-orders", CreateWorkOrder)
	rg.PUT("/work-orders/:id/status", UpdateWorkOrderStatus)
	rg.PUT("/work-orders/:id/assign", AssignWorkOrder)
	rg.DELETE("/work-orders/:id", DeleteWorkOrder)
	rg.GET("/work-orders/:id", GetWorkOrder)

	f := seedFault(801, model.FaultStatusOccurred)
	op := seedOperator("op_wo2")
	vw := model.User{Username: "vw_wo", PasswordHash: "x", Role: model.RoleViewer}
	model.DB.Create(&vw)

	// 故障不存在
	code, _ := doReq(t, r, "POST", "/api/v1/work-orders", `{"fault_id":99999,"device_hw_id":801}`)
	if code != http.StatusNotFound {
		t.Errorf("故障不存在应 404, got %d", code)
	}
	// 处理人不存在
	code, _ = doReq(t, r, "POST", "/api/v1/work-orders", `{"fault_id":`+uid(f.ID)+`,"device_hw_id":801,"assignee_id":99999}`)
	if code != http.StatusNotFound {
		t.Errorf("处理人不存在应 404, got %d", code)
	}
	// 处理人是 viewer
	code, _ = doReq(t, r, "POST", "/api/v1/work-orders", `{"fault_id":`+uid(f.ID)+`,"device_hw_id":801,"assignee_id":`+uid(vw.ID)+`}`)
	if code != http.StatusBadRequest {
		t.Errorf("viewer 处理人应 400, got %d", code)
	}
	// 缺参数
	code, _ = doReq(t, r, "POST", "/api/v1/work-orders", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺参数应 400, got %d", code)
	}
	// 正常创建
	code, body := doReq(t, r, "POST", "/api/v1/work-orders", `{"fault_id":`+uid(f.ID)+`,"device_hw_id":801,"assignee_id":`+uid(op.ID)+`}`)
	mustOK(t, code, body, "创建工单")
	wid := uint(body["data"].(map[string]interface{})["work_order"].(map[string]interface{})["id"].(float64))

	// 更新状态：无效状态
	code, _ = doReq(t, r, "PUT", "/api/v1/work-orders/"+uid(wid)+"/status", `{"status":"bogus"}`)
	if code != http.StatusBadRequest {
		t.Errorf("无效状态应 400, got %d", code)
	}
	// 更新状态：不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/work-orders/99999/status", `{"status":"processing"}`)
	if code != http.StatusNotFound {
		t.Errorf("不存在应 404, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "PUT", "/api/v1/work-orders/abc/status", `{"status":"processing"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
	// 派单：缺参数
	code, _ = doReq(t, r, "PUT", "/api/v1/work-orders/"+uid(wid)+"/assign", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺assignee应 400, got %d", code)
	}
	// 派单：处理人不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/work-orders/"+uid(wid)+"/assign", `{"assignee_id":99999}`)
	if code != http.StatusNotFound {
		t.Errorf("处理人不存在应 404, got %d", code)
	}
	// 派单：viewer
	code, _ = doReq(t, r, "PUT", "/api/v1/work-orders/"+uid(wid)+"/assign", `{"assignee_id":`+uid(vw.ID)+`}`)
	if code != http.StatusBadRequest {
		t.Errorf("viewer 派单应 400, got %d", code)
	}
	// 派单：工单不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/work-orders/99999/assign", `{"assignee_id":`+uid(op.ID)+`}`)
	if code != http.StatusNotFound {
		t.Errorf("工单不存在应 404, got %d", code)
	}
	// 派单成功（pending → processing）
	code, _ = doReq(t, r, "PUT", "/api/v1/work-orders/"+uid(wid)+"/assign", `{"assignee_id":`+uid(op.ID)+`}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "派单")
	// 详情不存在
	code, _ = doReq(t, r, "GET", "/api/v1/work-orders/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("详情不存在应 404, got %d", code)
	}
	// 删除不存在
	code, _ = doReq(t, r, "DELETE", "/api/v1/work-orders/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("删除不存在应 404, got %d", code)
	}
	// 删除非法ID
	code, _ = doReq(t, r, "DELETE", "/api/v1/work-orders/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("删除非法ID应 400, got %d", code)
	}
}

func TestWorkOrder_RejectReprocess(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.POST("/work-orders", CreateWorkOrder)
	rg.PUT("/work-orders/:id/status", UpdateWorkOrderStatus)
	f := seedFault(901, model.FaultStatusOccurred)
	_, body := doReq(t, r, "POST", "/api/v1/work-orders", `{"fault_id":`+uid(f.ID)+`,"device_hw_id":901}`)
	wid := uint(body["data"].(map[string]interface{})["work_order"].(map[string]interface{})["id"].(float64))
	// 驳回
	code, _ := doReq(t, r, "PUT", "/api/v1/work-orders/"+uid(wid)+"/status", `{"status":"rejected"}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "驳回")
	// 重新派发处理中→处理中并设closed_at nil（rejected→pending 分支）
	code, _ = doReq(t, r, "PUT", "/api/v1/work-orders/"+uid(wid)+"/status", `{"status":"pending"}`)
	if code != http.StatusOK {
		t.Errorf("rejected→pending 应 200, got %d", code)
	}
}
