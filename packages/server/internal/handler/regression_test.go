package handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// 回归测试：针对 refactor-notes.md 中 handler 层改动
//   基线：origin/main a460365 ｜ 范围：不改变业务行为
// ============================================================================

// TestRegression_C1_RejectToPendingMerge 覆盖 C1：UpdateWorkOrderStatus 的 rejected→pending
// 分支合并后，语义必须保持：工单回 pending、closed_at 清空、关联故障回到 confirmed。
// 本测试在现有 TestWorkOrder_RejectReprocess（仅断言 200）基础上强化校验。
func TestRegression_C1_RejectToPendingMerge(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.POST("/work-orders", CreateWorkOrder)
	rg.PUT("/work-orders/:id/status", UpdateWorkOrderStatus)

	f := seedFault(9501, model.FaultStatusOccurred)
	_, body := doReq(t, r, "POST", "/api/v1/work-orders",
		`{"fault_id":`+uid(f.ID)+`,"device_hw_id":9501}`)
	wid := uint(body["data"].(map[string]interface{})["work_order"].(map[string]interface{})["id"].(float64))

	// rejected → 工单应为 rejected、closed_at 被填（模拟曾关闭）
	doReq(t, r, "PUT", "/api/v1/work-orders/"+uid(wid)+"/status", `{"status":"rejected"}`)
	// 手动设置 closed_at 以模拟历史关闭状态（确认重派时会清空）
	nowTS := time.Now()
	model.DB.Model(&model.WorkOrder{}).Where("id = ?", wid).Update("closed_at", nowTS)

	// rejected → pending：C1 合并分支应清空 closed_at + 故障回 confirmed
	code, _ := doReq(t, r, "PUT", "/api/v1/work-orders/"+uid(wid)+"/status", `{"status":"pending"}`)
	if code != 200 {
		t.Fatalf("rejected→pending 应 200, got %d", code)
	}

	var wo model.WorkOrder
	model.DB.First(&wo, wid)
	if wo.Status != model.WorkOrderStatusPending {
		t.Errorf("工单状态 = %s, 期望 pending", wo.Status)
	}
	if wo.ClosedAt != nil {
		t.Errorf("rejected→pending 后 ClosedAt 应被清空, got %v", wo.ClosedAt)
	}

	// 关联故障应回到 confirmed
	var fault model.FaultRecord
	model.DB.First(&fault, f.ID)
	if fault.Status != model.FaultStatusConfirmed {
		t.Errorf("rejected→pending 后故障状态 = %s, 期望 confirmed", fault.Status)
	}
}

// registerWorkOrderRoutes 注册工单列表/状态路由
func registerWorkOrderRoutes(r *gin.Engine) {
	rg := r.Group("/api/v1")
	rg.GET("/work-orders", ListWorkOrders)
	rg.PUT("/work-orders/:id/status", UpdateWorkOrderStatus)
}

// TestRegression_B1_ListWorkOrdersPreloadNames 覆盖 B1：ListWorkOrders 批量预取后
// assignee_name 字段必须与逐行查询一致（含存在/不存在处理人两种情况）。
func TestRegression_B1_ListWorkOrdersPreloadNames(t *testing.T) {
	r := covSetup(t)
	registerWorkOrderRoutes(r)

	owner := seedOperator("b1_owner")
	f1 := seedFault(9601, model.FaultStatusOccurred)
	f2 := seedFault(9602, model.FaultStatusOccurred)
	ghost := uint(88888)

	mkWO := func(faultID uint, assignee *uint) uint {
		wo := model.WorkOrder{OrderNo: model.NextOrderNo(model.DB), FaultID: faultID,
			DeviceHwID: 1, Status: model.WorkOrderStatusPending, AssigneeID: assignee}
		model.DB.Create(&wo)
		return wo.ID
	}
	// 三条工单：有处理人、无处理人、处理人指向不存在用户
	_ = mkWO(f1.ID, &owner.ID)                                      // assignee_name 应为 b1_owner
	_ = mkWO(f2.ID, nil)                                            // 无处理人 → 空
	_ = mkWO(seedFault(9603, model.FaultStatusOccurred).ID, &ghost) // ghost → 空

	code, body := doReq(t, r, "GET", "/api/v1/work-orders?page_size=50", "")
	if code != 200 {
		t.Fatalf("列表应 200, got %d", code)
	}
	list := body["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) < 3 {
		t.Fatalf("列表长度 = %d, 期望 ≥3", len(list))
	}

	// 校验每个工单 id 对应的 assignee_name 与用户表一致
	for _, item := range list {
		it := item.(map[string]interface{})
		wid := uint(it["id"].(float64))
		var wo model.WorkOrder
		model.DB.First(&wo, wid)
		expected := ""
		if wo.AssigneeID != nil {
			var u model.User
			if err := model.DB.Select("id, username").First(&u, *wo.AssigneeID).Error; err == nil {
				expected = u.Username
			}
		}
		if expected != "" && it["assignee_name"] != expected {
			t.Errorf("工单 %d assignee_name=%v, 期望 %q", wid, it["assignee_name"], expected)
		}
	}

	// 显式校验 b1_owner 的 name 出现，ghost 处理人 name 为空
	foundOwner, foundGhost := false, false
	for _, item := range list {
		it := item.(map[string]interface{})
		if it["assignee_name"] == "b1_owner" {
			foundOwner = true
		}
		wid := uint(it["id"].(float64))
		var wo model.WorkOrder
		model.DB.First(&wo, wid)
		if wo.AssigneeID != nil && *wo.AssigneeID == ghost && it["assignee_name"] != "" {
			foundGhost = true
		}
	}
	if !foundOwner {
		t.Error("存在处理人时 assignee_name 应返回 'b1_owner'")
	}
	if foundGhost {
		t.Error("处理人不存在时 assignee_name 应为空")
	}
}

// registerFaultListRoute 注册故障列表路由
func registerFaultListRoute(r *gin.Engine) {
	rg := r.Group("/api/v1")
	rg.GET("/faults", ListFaults)
}

// TestRegression_B1_ListFaultsPreloadNames 覆盖 B1：ListFaults 批量预取后 owner_name /
// repairer_name 与逐行查询一致（含存在/不存在的负责人、维修人情况）。
func TestRegression_B1_ListFaultsPreloadNames(t *testing.T) {
	r := covSetup(t)
	registerFaultListRoute(r)

	owner := seedOperator("fault_owner")
	repairer := seedOperator("fault_repairer")
	f1 := model.FaultRecord{DeviceHwID: 9701, ErrCode: -1, FaultType: "lamp_off",
		FaultLevel: "critical", Status: model.FaultStatusOccurred,
		OwnerID: &owner.ID, RepairerID: &repairer.ID}
	model.DB.Create(&f1)
	f2 := model.FaultRecord{DeviceHwID: 9702, ErrCode: -2, FaultType: "lamp_off",
		FaultLevel: "critical", Status: model.FaultStatusOccurred} // 无负责人/维修人
	model.DB.Create(&f2)
	ghost := uint(88887)
	f3 := model.FaultRecord{DeviceHwID: 9703, ErrCode: -3, FaultType: "lamp_off",
		FaultLevel: "critical", Status: model.FaultStatusOccurred, OwnerID: &ghost}
	model.DB.Create(&f3)

	code, body := doReq(t, r, "GET", "/api/v1/faults?page_size=50", "")
	if code != 200 {
		t.Fatalf("列表应 200, got %d", code)
	}
	list := body["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) < 3 {
		t.Fatalf("列表长度 = %d, 期望 ≥3", len(list))
	}

	for _, item := range list {
		it := item.(map[string]interface{})
		fid := uint(it["id"].(float64))
		var fmodel model.FaultRecord
		model.DB.First(&fmodel, fid)

		expOwner, expRepairer := "", ""
		if fmodel.OwnerID != nil {
			var u model.User
			if err := model.DB.Select("id, username").First(&u, *fmodel.OwnerID).Error; err == nil {
				if u.Username != "" {
					expOwner = u.Username
				}
			}
		}
		if fmodel.RepairerID != nil {
			var u model.User
			if err := model.DB.Select("id, username").First(&u, *fmodel.RepairerID).Error; err == nil {
				if u.Username != "" {
					expRepairer = u.Username
				}
			}
		}
		gotOwner := fmt.Sprint(it["owner_name"])
		gotRepairer := fmt.Sprint(it["repairer_name"])
		if expOwner == "" {
			// 无负责人/无姓名：字段应缺省（nil 或空），与逐行 faultView 一致
			if v, ok := it["owner_name"]; ok && v != nil && v != "" {
				t.Errorf("故障 %d 无负责人但 owner_name=%v, 期望 空", fid, v)
			}
		} else if gotOwner != expOwner {
			t.Errorf("故障 %d owner_name=%v, 期望 %q", fid, it["owner_name"], expOwner)
		}
		if expRepairer == "" {
			if v, ok := it["repairer_name"]; ok && v != nil && v != "" {
				t.Errorf("故障 %d 无维修人但 repairer_name=%v, 期望 空", fid, v)
			}
		} else if gotRepairer != expRepairer {
			t.Errorf("故障 %d repairer_name=%v, 期望 %q", fid, it["repairer_name"], expRepairer)
		}
	}
}

// TestRegression_B1_ActiveStatusFilter 覆盖 B5/behavior red line：active 兼容语义
// （occurred/confirmed/dispatched）在重构为 ParseStatusFilter 后保持不变。
func TestRegression_B1_ActiveStatusFilter(t *testing.T) {
	r := covSetup(t)
	registerFaultListRoute(r)

	seedFault(9801, model.FaultStatusOccurred)
	seedFault(9802, model.FaultStatusConfirmed)
	seedFault(9803, model.FaultStatusDispatched)
	seedFault(9804, model.FaultStatusResolved)

	// active 应命中前三条未解决，不含 resolved
	code, body := doReq(t, r, "GET", "/api/v1/faults?status=active", "")
	if code != 200 {
		t.Fatalf("active 筛选应 200, got %d", code)
	}
	total := int(body["data"].(map[string]interface{})["total"].(float64))
	if total != 3 {
		t.Errorf("active 筛选 total = %d, 期望 3(occurred/confirmed/dispatched)", total)
	}
	// resolved 精确匹配
	code, body = doReq(t, r, "GET", "/api/v1/faults?status=resolved", "")
	if code != 200 {
		t.Fatalf("resolved 筛选应 200, got %d", code)
	}
	total = int(body["data"].(map[string]interface{})["total"].(float64))
	if total != 1 {
		t.Errorf("resolved 筛选 total = %d, 期望 1", total)
	}
}
