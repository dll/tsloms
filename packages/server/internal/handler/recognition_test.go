package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// registerRecognitionRoutes 注册研判/证据/案例路由
func registerRecognitionRoutes(r *gin.Engine) {
	rg := r.Group("/api/v1")
	rg.GET("/faults/:id/evidence", ListFaultEvidence)
	rg.POST("/evidence/ingest", IngestEvidence)
	rg.GET("/evidence/sources", ListEvidenceSources)
	rg.GET("/fault-cases", ListFaultCases)
	rg.POST("/fault-cases", CreateFaultCase)
	rg.POST("/fault-cases/train", TrainFaultCases)
	rg.GET("/recognition/stats", RecognitionStats)
	rg.POST("/faults/:id/review", ReviewFault)
}

// TestFaultEvidenceList 证据明细接口：按 fault 与末次研判批次可回看
func TestFaultEvidenceList(t *testing.T) {
	r := covSetup(t)
	registerRecognitionRoutes(r)

	f := seedFault(4001, model.FaultStatusOccurred)
	// 落一条主信号证据
	ev := model.FaultEvidence{
		FaultID: &f.ID, EvaluationID: "batch-1", DeviceHwID: 4001,
		SourceType: model.EvSourceFirmware, CapturedAt: now(),
	}
	model.DB.Create(&ev)
	model.DB.Model(&f).Update("last_evaluation_id", "batch-1")

	code, body := doReq(t, r, "GET", "/api/v1/faults/"+uid(f.ID)+"/evidence", "")
	mustOK(t, code, body, "证据明细")
	total := int(body["data"].(map[string]interface{})["total"].(float64))
	if total < 1 {
		t.Errorf("证据明细 total 应≥1, 实际 %d", total)
	}
}

// TestEvidenceIngest 预留证据写入接口
func TestEvidenceIngest(t *testing.T) {
	r := covSetup(t)
	registerRecognitionRoutes(r)

	code, body := doReq(t, r, "POST", "/api/v1/evidence/ingest",
		`{"device_hw_id":4002,"source_type":"citizen","raw_data":"群众反映红灯不亮"}`)
	mustOK(t, code, body, "注入群众反映证据")
	ev := body["data"].(map[string]interface{})["evidence"].(map[string]interface{})
	if ev["source_type"] != "citizen" {
		t.Errorf("注入证据 source_type=%v, 期望 citizen", ev["source_type"])
	}

	// 非法 source_type → 400
	code, _ = doReq(t, r, "POST", "/api/v1/evidence/ingest", `{"device_hw_id":4002,"source_type":"bad"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法 source_type 应 400, got %d", code)
	}
	// 缺必填 → 400
	code, _ = doReq(t, r, "POST", "/api/v1/evidence/ingest", `{"source_type":"citizen"}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺 device_hw_id 应 400, got %d", code)
	}
}

// TestEvidenceSources 证据来源枚举
func TestEvidenceSources(t *testing.T) {
	r := covSetup(t)
	registerRecognitionRoutes(r)
	code, body := doReq(t, r, "GET", "/api/v1/evidence/sources", "")
	mustOK(t, code, body, "来源枚举")
	if body["data"].(map[string]interface{})["list"] == nil {
		t.Error("应返回来源枚举 list")
	}
}

// TestFaultCasesCRUD 案例库列表 / 人工回标 / 训练 / 统计
func TestFaultCasesCRUD(t *testing.T) {
	r := covSetup(t)
	registerRecognitionRoutes(r)

	// 人工回标一条案例
	code, body := doReq(t, r, "POST", "/api/v1/fault-cases",
		`{"device_hw_id":4003,"fault_type":"lamp_off","fault_level":"critical","expected_result":"lamp_off","judged_result":"lamp_off"}`)
	mustOK(t, code, body, "人工回标案例")
	if body["data"].(map[string]interface{})["case"] == nil {
		t.Error("应返回回标案例")
	}

	// 列表
	code, body = doReq(t, r, "GET", "/api/v1/fault-cases?fault_type=lamp_off", "")
	mustOK(t, code, body, "案例列表")
	if body["data"].(map[string]interface{})["total"].(float64) < 1 {
		t.Errorf("案例列表 total 应≥1")
	}

	// 训练
	code, _ = doReq(t, r, "POST", "/api/v1/fault-cases/train", "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "案例训练")

	// 统计
	code, body = doReq(t, r, "GET", "/api/v1/recognition/stats", "")
	mustOK(t, code, body, "识别统计")
	data := body["data"].(map[string]interface{})
	if _, ok := data["accuracy"]; !ok {
		t.Error("识别统计应含 accuracy")
	}
	if data["total_cases"].(float64) < 1 {
		t.Errorf("total_cases 应≥1")
	}
}

// TestReviewFault 待确认复核：升级确认 critical 自动派单
func TestReviewFault(t *testing.T) {
	r := covSetup(t)
	registerRecognitionRoutes(r)

	// 造一条待确认的 critical 故障（模拟引擎判 pending_review 未派单）
	f := seedFault(4004, model.FaultStatusOccurred)
	high := 0.6
	f.FaultLevel = "critical"
	f.RecognitionStatus = model.RecognitionPendingReview
	f.Confidence = &high
	model.DB.Save(&f)

	code, body := doReq(t, r, "POST", "/api/v1/faults/"+uid(f.ID)+"/review", `{"confirmed":true}`)
	mustOK(t, code, body, "复核确认真故障")
	ft := body["data"].(map[string]interface{})["fault"].(map[string]interface{})
	if ft["recognition_status"] != model.RecognitionConfirmed {
		t.Errorf("复核后识别状态=%v, 期望 confirmed", ft["recognition_status"])
	}
	// critical 确认真故障 → 自动派单
	var woCount int64
	model.DB.Model(&model.WorkOrder{}).Where("fault_id = ?", f.ID).Count(&woCount)
	if woCount != 1 {
		t.Errorf("复核确认真故障 critical 应自动派单, 实际 %d", woCount)
	}
}

// TestReviewFault_MarkFalsePositive 复核判为误报：标记误报、不派单
func TestReviewFault_MarkFalsePositive(t *testing.T) {
	r := covSetup(t)
	registerRecognitionRoutes(r)
	f := seedFault(4005, model.FaultStatusOccurred)
	f.RecognitionStatus = model.RecognitionPendingReview
	f.FaultLevel = "critical"
	model.DB.Save(&f)

	code, _ := doReq(t, r, "POST", "/api/v1/faults/"+uid(f.ID)+"/review", `{"confirmed":false}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "复核判误报")
	var woCount int64
	model.DB.Model(&model.WorkOrder{}).Where("fault_id = ?", f.ID).Count(&woCount)
	if woCount != 0 {
		t.Errorf("判误报不应派单, 实际 %d", woCount)
	}
}
