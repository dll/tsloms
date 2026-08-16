package handler

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// 范围B 回归：ListFaults 的 recognition_status 可选筛选（case B8）
//   - recognition_status=active  → 兼容旧语义：未解决三态（occurred/confirmed/dispatched）
//   - recognition_status=<literal> → 字面匹配 recognition_status 列
//   - 不带该参数 → 行为完全不变（向后兼容，返回全部）
// ============================================================================

func seedRecognitionFault(hw uint32, status, recogStatus string) model.FaultRecord {
	now := time.Now()
	conf := 0.9
	f := model.FaultRecord{
		DeviceHwID: hw, ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical",
		Status: status, FirstSeen: now, LastSeen: now,
		Confidence: &conf, RecognitionStatus: recogStatus,
	}
	model.DB.Create(&f)
	return f
}

// TestB8_ListFaultsRecognitionStatusFilter 验证 recognition_status 筛选生效且无参时不变
func TestB8_ListFaultsRecognitionStatusFilter(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/faults", ListFaults)

	// 数据：3 条未解决(occurred/confirmed/dispatched)不同识别状态 + 1 条 resolved
	seedRecognitionFault(501, model.FaultStatusOccurred, model.RecognitionConfirmed)
	seedRecognitionFault(502, model.FaultStatusConfirmed, model.RecognitionConfirmed)
	seedRecognitionFault(503, model.FaultStatusDispatched, model.RecognitionPendingReview)
	seedRecognitionFault(504, model.FaultStatusResolved, model.RecognitionFiltered)

	// 1) 无参 → 返回全部 4 条（向后兼容不变）
	code, body := doReq(t, r, "GET", "/api/v1/faults", "")
	mustOK(t, code, body, "无参列表")
	total := int(body["data"].(map[string]interface{})["total"].(float64))
	if total != 4 {
		t.Errorf("无参应返回全部 4 条, got %d", total)
	}

	// 2) recognition_status=active → 未解决三态（含 confirmed 与 pending_review，不含 resolved）
	code, body = doReq(t, r, "GET", "/api/v1/faults?recognition_status=active", "")
	mustOK(t, code, body, "active 筛选")
	list := body["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 3 {
		t.Fatalf("active 应命中未解决 3 条, got %d", len(list))
	}
	for _, it := range list {
		itm := it.(map[string]interface{})
		st := itm["status"].(string)
		if st == model.FaultStatusResolved {
			t.Errorf("active 不应包含 resolved, got %v", st)
		}
	}

	// 3) recognition_status=pending_review → 字面匹配 1 条
	code, body = doReq(t, r, "GET", "/api/v1/faults?recognition_status=pending_review", "")
	mustOK(t, code, body, "pending_review 筛选")
	data := body["data"].(map[string]interface{})
	if int(data["total"].(float64)) != 1 {
		t.Errorf("pending_review 应命中 1 条, got %v", data["total"])
	}
	if data["list"].([]interface{})[0].(map[string]interface{})["device_hw_id"].(float64) != 503 {
		t.Errorf("pending_review 命中设备应 503")
	}

	// 4) recognition_status=filtered → 命中 1 条（resolved 的那条）
	code, body = doReq(t, r, "GET", "/api/v1/faults?recognition_status=filtered", "")
	mustOK(t, code, body, "filtered 筛选")
	if int(body["data"].(map[string]interface{})["total"].(float64)) != 1 {
		t.Errorf("filtered 应命中 1 条, got %v", body["data"].(map[string]interface{})["total"])
	}

	// 5) recognition_status 不影响 status= 组合筛选（兼容并存）
	code, body = doReq(t, r, "GET", "/api/v1/faults?status=confirmed&recognition_status=confirmed", "")
	mustOK(t, code, body, "组合筛选")
	if int(body["data"].(map[string]interface{})["total"].(float64)) != 1 {
		t.Errorf("status=confirmed+recognition confirmed 应命中 1 条")
	}
}

// TestB8_ListFaultViewIncludesRecognitionFields 验证 faultView 携带识别可选字段（前端消费）
func TestB8_ListFaultViewIncludesRecognitionFields(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/faults/:id", GetFault)

	f := seedRecognitionFault(505, model.FaultStatusOccurred, model.RecognitionConfirmed)
	code, body := doReq(t, r, "GET", "/api/v1/faults/"+uid(f.ID), "")
	mustOK(t, code, body, "详情")
	ft := body["data"].(map[string]interface{})["fault"].(map[string]interface{})

	// 新增可选字段带缺省（null/空可接受，但不能缺失导致前端 undefined 崩）
	for _, k := range []string{"confidence", "recognition_source", "recognition_status",
		"evidence_count", "last_evaluation_id"} {
		if _, ok := ft[k]; !ok {
			t.Errorf("faultView 应含识别字段 %s", k)
		}
	}
	if ft["recognition_status"] != model.RecognitionConfirmed {
		t.Errorf("recognition_status 应 confirmed, got %v", ft["recognition_status"])
	}
	if ft["confidence"] == nil {
		t.Errorf("confidence 应非空")
	}
}

// TestB8_ReviewNormalNotAutoDispatch 复核：normal 等级确认真故障不回自动派单（R6 只 critical 派单）
func TestB8_ReviewNormalNotAutoDispatch(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.POST("/faults/:id/review", ReviewFault)

	f := seedRecognitionFault(506, model.FaultStatusOccurred, model.RecognitionPendingReview)
	conf := 0.6
	f.FaultLevel = "normal"
	f.Confidence = &conf
	model.DB.Save(&f)

	code, _ := doReq(t, r, "POST", "/api/v1/faults/"+uid(f.ID)+"/review", `{"confirmed":true}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "复核 normal")
	var woCount int64
	model.DB.Model(&model.WorkOrder{}).Where("fault_id = ?", f.ID).Count(&woCount)
	if woCount != 0 {
		t.Errorf("normal 等级复核确认不应自动派单, 实际 %d", woCount)
	}
}

// TestB8_ReviewPendingCriticalDispatchOnce 复核：critical 未派单 → 自动派单一笔；重复复核不重复派单
func TestB8_ReviewPendingCriticalDispatchOnce(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.POST("/faults/:id/review", ReviewFault)

	f := seedRecognitionFault(507, model.FaultStatusOccurred, model.RecognitionPendingReview)
	conf := 0.6
	f.FaultLevel = "critical"
	f.Confidence = &conf
	model.DB.Save(&f)

	// 确认真故障 critical → 自动派单
	code, _ := doReq(t, r, "POST", "/api/v1/faults/"+uid(f.ID)+"/review", `{"confirmed":true}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "复核确认 critical")
	var wo model.WorkOrder
	model.DB.Where("fault_id = ?", f.ID).First(&wo)
	if wo.ID == 0 {
		t.Fatal("critical 复核确认应自动派单")
	}
	// 复核后 fault 应带 work_order_id 且状态 confirmed
	var ff model.FaultRecord
	model.DB.First(&ff, f.ID)
	if ff.WorkOrderID == nil || *ff.WorkOrderID != wo.ID {
		t.Errorf("复核后 fault 应回写 work_order_id, got %v", ff.WorkOrderID)
	}
	if ff.Status != model.FaultStatusConfirmed {
		t.Errorf("复核后 status 应 confirmed, got %s", ff.Status)
	}

	// 再次复核应不再生成新工单（幂等，避免重复派单）
	var woCount int64
	model.DB.Model(&model.WorkOrder{}).Where("fault_id = ?", f.ID).Count(&woCount)
	before := woCount
	code, _ = doReq(t, r, "POST", "/api/v1/faults/"+uid(f.ID)+"/review", `{"confirmed":true}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "二次复核")
	model.DB.Model(&model.WorkOrder{}).Where("fault_id = ?", f.ID).Count(&woCount)
	if woCount != before {
		t.Errorf("重复复核不应重复派单, before=%d after=%d", before, woCount)
	}
}
