package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/caselib"
	"github.com/tsloms/server/internal/model"
	"github.com/tsloms/server/internal/recognition"
)

// ============================================================================
// 智能多源故障识别研判引擎 —— REST 接口（范围A，独立路径，向后兼容）
//   · GET  /faults/:id/evidence      —— 拉取某起故障的多源证据明细
//   · POST /evidence/ingest          —— 预留外部数据源证据写入（举证/反馈/监控）
//   · GET  /evidence/sources         —— 多源证据类型/来源枚举
//   · POST /fault-cases              —— 案例库新增/人工回标（可选，亦可仅由引擎自动沉淀）
//   · GET  /fault-cases              —— 案例库检索/列表
//   · POST /fault-cases/train        —— 触发案例库训练（骨架，向 100% 识别率收敛）
//   · GET  /recognition/stats        —— 识别准确率/误报/漏报/置信度分布
//   · POST /faults/:id/review        —— 待确认复核：证据补充后升级为确认（可自动派单）
// 既有 /faults*、/work-orders* 契约不变（R9）；新增全部走独立路径。
// ============================================================================

// ListFaultEvidence 拉取某起故障的多源证据明细（含被过滤的研判批次证据亦可按 evaluation 查）
func ListFaultEvidence(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "故障ID无效")
		return
	}
	var fault model.FaultRecord
	if err := model.DB.First(&fault, id).Error; err != nil {
		notFound(c, "故障记录不存在")
		return
	}

	// 优先按 fault_id 拉证据；兼容按末次研判批次号（保证被过滤证据可回看）
	var list []model.FaultEvidence
	q := model.DB.Where("fault_id = ?", id)
	if fault.LastEvaluationID != "" {
		q = model.DB.Where("evaluation_id = ?", fault.LastEvaluationID)
		// 取并集：该故障关联 + 末次研判批次（evaluation 能覆盖 fault 主信号证据）
		var merged []model.FaultEvidence
		model.DB.Where("fault_id = ? OR evaluation_id = ?", id, fault.LastEvaluationID).
			Order("captured_at ASC").Find(&merged)
		list = merged
	} else {
		q.Order("captured_at ASC").Find(&list)
	}

	ok(c, gin.H{"fault_id": id, "list": list, "total": len(list)})
}

// IngestEvidence 预留外部数据源证据写入（内部归一化落 fault_evidence）
// body: device_hw_id, source_type, err_code?, led_state?, current_r/y/g?, raw_data?,
//       ref_media_id?, ref_feedback_id?, captured_at?, fault_id?
func IngestEvidence(c *gin.Context) {
	var req struct {
		DeviceHwID   uint32  `json:"device_hw_id" binding:"required"`
		SourceType   string  `json:"source_type" binding:"required"`
		ErrCode      *int8   `json:"err_code"`
		LedState     *int8   `json:"led_state"`
		CurrentR     *uint16 `json:"current_r"`
		CurrentY     *uint16 `json:"current_y"`
		CurrentG     *uint16 `json:"current_g"`
		RawData      string  `json:"raw_data"`
		RefMediaID   *uint   `json:"ref_media_id"`
		RefFeedbackID *uint  `json:"ref_feedback_id"`
		CapturedAt   *time.Time `json:"captured_at"`
		FaultID      *uint   `json:"fault_id"`
		Confidence   *float64 `json:"confidence"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（device_hw_id、source_type 必填）")
		return
	}
	// source_type 白名单校验
	if !validSourceType(req.SourceType) {
		badRequest(c, "source_type 不合法（firmware/current/led_state/citizen/photo_evidence/video_monitor）")
		return
	}

	ev := recognition.RuleEvidence{
		DeviceHwID:    req.DeviceHwID,
		SourceType:    req.SourceType,
		ErrCode:       req.ErrCode,
		LedState:      req.LedState,
		CurrentR:      req.CurrentR,
		CurrentY:      req.CurrentY,
		CurrentG:      req.CurrentG,
		RawData:       req.RawData,
		RefMediaID:    req.RefMediaID,
		RefFeedbackID: req.RefFeedbackID,
		Confidence:    0.8,
	}
	if req.Confidence != nil {
		ev.Confidence = *req.Confidence
	}
	captured := req.CapturedAt
	if captured == nil {
		now := time.Now()
		captured = &now
	}
	ev.CapturedAt = *captured

	// 独立 evidence 事件：无 fault_id时给一个临时批次号，保证可溯源
	evID := fmt.Sprintf("ingest-%d-%d", req.DeviceHwID, time.Now().UnixNano())
	m := recognition.EvidenceToModel(ev, req.FaultID, evID)
	if err := model.DB.Create(&m).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("evidence/%d", m.ID), "注入多源证据 "+req.SourceType)
	ok(c, gin.H{"evidence": m, "message": "证据已写入"})
}

// ListEvidenceSources 多源证据类型/来源枚举（供前端下拉；本阶段前端可不使用）
func ListEvidenceSources(c *gin.Context) {
	ok(c, gin.H{"list": model.EvidenceSourceTypes})
}

// ListFaultCases 案例库检索/列表（供长尾排查与训练状态观测）
// 支持按设备、故障类型、状态、正确性筛选，分页
func ListFaultCases(c *gin.Context) {
	page, pageSize := paginate(c)
	q := model.DB.Model(&model.FaultCase{})
	if hwID := c.Query("device_hw_id"); hwID != "" {
		q = q.Where("device_hw_id = ?", hwID)
	}
	if ft := c.Query("fault_type"); ft != "" {
		q = q.Where("fault_type = ?", ft)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if corr := c.Query("is_correct"); corr == "true" {
		q = q.Where("is_correct = ?", true)
	} else if corr == "false" {
		q = q.Where("is_correct = ?", false)
	}

	var total int64
	q.Count(&total)
	var list []model.FaultCase
	q.Order("created_at DESC").Offset(int((page-1)*pageSize)).Limit(int(pageSize)).Find(&list)
	ok(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// CreateFaultCase 案例库新增/人工回标样本（管理员/运维）
// body: device_hw_id, input_signature?, fault_type, fault_level, expected_result?,
//       evidence_summary?, source_evaluation_id?
func CreateFaultCase(c *gin.Context) {
	var req struct {
		DeviceHwID        uint32 `json:"device_hw_id" binding:"required"`
		InputSignature    string `json:"input_signature"`
		FaultType         string `json:"fault_type"`
		FaultLevel        string `json:"fault_level"`
		ExpectedResult    string `json:"expected_result"`
		JudgedResult      string `json:"judged_result"`
		EvidenceSummary   string `json:"evidence_summary"`
		SourceEvaluationID string `json:"source_evaluation_id"`
		Status            string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（device_hw_id 必填）")
		return
	}
	isCorrect := false
	if req.ExpectedResult != "" {
		isCorrect = req.ExpectedResult == req.JudgedResult
	}
	cs := model.FaultCase{
		DeviceHwID:         req.DeviceHwID,
		InputSignature:     req.InputSignature,
		FaultType:          req.FaultType,
		FaultLevel:         req.FaultLevel,
		ExpectedResult:     req.ExpectedResult,
		JudgedResult:       req.JudgedResult,
		IsCorrect:          &isCorrect,
		EvidenceSummary:    req.EvidenceSummary,
		SourceEvaluationID: req.SourceEvaluationID,
		Status:             model.CaseStatusConfirmed,
	}
	if req.Status != "" {
		cs.Status = req.Status
	}
	if err := model.DB.Create(&cs).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("fault-case/%d", cs.ID), "人工回标案例")
	ok(c, gin.H{"case": cs, "message": "案例已写入"})
}

// TrainFaultCases 触发案例库模型训练到 100% 识别率达标（骨架）
func TrainFaultCases(c *gin.Context) {
	cr := caselib.NewCaseRecorder(model.DB)
	res, err := cr.Train()
	if err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, "fault-case/train", "触发案例库训练")
	ok(c, res)
}

// RecognitionStats 识别准确率/误报/漏报/置信度分布统计
func RecognitionStats(c *gin.Context) {
	cr := caselib.NewCaseRecorder(model.DB)
	res, err := cr.Stats()
	if err != nil {
		serverError(c, err)
		return
	}
	ok(c, res)
}

// ReviewFault 待确认复核：证据补充后升级为确认（可自动派单）。
// body: (可选) evidence 追加注入由 evidence/ingest 单独完成；本接口仅做状态升级 + 置信度回写。
func ReviewFault(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "故障ID无效")
		return
	}
	var req struct {
		Confirmed bool `json:"confirmed"` // true=确认真故障，false=标记误报
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	var fault model.FaultRecord
	if err := model.DB.First(&fault, id).Error; err != nil {
		notFound(c, "故障记录不存在")
		return
	}

	now := time.Now()
	var highConf float64 = 0.99 // 人工复核通过 → 高置信回写
	var falsePositive bool = !req.Confirmed

	updates := map[string]interface{}{
		"recognition_status": model.RecognitionConfirmed,
		"confidence":         highConf,
		"is_false_positive":  falsePositive,
		"reviewed_at":        &now,
	}
	if !req.Confirmed {
		updates["recognition_status"] = model.RecognitionFiltered
	}
	if err := model.DB.Model(&fault).Updates(updates).Error; err != nil {
		serverError(c, err)
		return
	}

	// 复核确认为真故障：若为 critical 且未自动派单（此前 pending_review 未派），则自动派单。
	// M1：委托 model.EnsureActiveWorkOrder 原子式防重（配合活跃工单部分唯一索引），并发复核也只建成一条。
	if req.Confirmed && fault.FaultLevel == "critical" {
		model.EnsureActiveWorkOrder(model.DB, fault.ID, fault.DeviceHwID)
	}

	// 同步更新案例库：该批次案例标记为正确性回标
	if fault.LastEvaluationID != "" {
		model.DB.Model(&model.FaultCase{}).
			Where("source_evaluation_id = ?", fault.LastEvaluationID).
			Update("is_correct", req.Confirmed)
	}

	model.DB.First(&fault, id)
	recordOperation(c, model.OpUpdate, fmt.Sprintf("fault/%d", fault.ID), "待确认复核:"+fault.RecognitionStatus)
	ok(c, gin.H{"fault": faultView(c, fault), "message": "复核完成"})
}

func validSourceType(s string) bool {
	for _, v := range model.EvidenceSourceTypes {
		if v == s {
			return true
		}
	}
	return false
}
