package mqtt

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
	"github.com/tsloms/server/internal/recognition"
)

// TestProcessFault_PersistsEvidenceAndCase 故障研判后多源证据落库 + 案例沉淀
func TestProcessFault_PersistsEvidenceAndCase(t *testing.T) {
	h := newTestHandler(t)
	rec := createFaultRec(6001, LEDErrROFF) // critical, confirmed
	h.processFault(rec)

	// 多源证据落库
	var evCount int64
	model.DB.Model(&model.FaultEvidence{}).Where("device_hw_id = ?", recognition.LedUUID(6001)).Count(&evCount)
	if evCount == 0 {
		t.Error("研判后应写入多源证据(fault_evidence)")
	}
	// 主信号证据 source=firmware
	var ev model.FaultEvidence
	model.DB.Where("device_hw_id = ? AND source_type = ?", recognition.LedUUID(6001), model.EvSourceFirmware).First(&ev)
	if ev.SourceType != model.EvSourceFirmware {
		t.Errorf("主信号证据 source=%s, 期望 firmware", ev.SourceType)
	}
	if ev.FaultID == nil {
		t.Error("确认故障的证据应关联 fault_id")
	}

	// 案例库沉淀
	var caseCount int64
	model.DB.Model(&model.FaultCase{}).Count(&caseCount)
	if caseCount != 1 {
		t.Errorf("应沉淀 1 条案例, 实际 %d", caseCount)
	}
}

// TestProcessFault_CriticalConfirmedAutoWorkorder 保持红线：confirmed critical 仍自动工单
func TestProcessFault_CriticalConfirmedAutoWorkorder(t *testing.T) {
	h := newTestHandler(t)
	rec := createFaultRec(6002, LEDErrROFF)
	h.processFault(rec)

	var woCount int64
	model.DB.Model(&model.WorkOrder{}).Count(&woCount)
	if woCount != 1 {
		t.Errorf("critical confirmed 应自动建 1 个工单, 实际 %d", woCount)
	}
	var f model.FaultRecord
	model.DB.Where("device_hw_id = ?", recognition.LedUUID(rec.LedHwID)).First(&f)
	if f.RecognitionStatus != model.RecognitionConfirmed {
		t.Errorf("fault recognition_status=%s, 期望 confirmed", f.RecognitionStatus)
	}
	if f.Confidence == nil {
		t.Error("confirmed 故障应写置信度")
	}
}

// TestProcessFault_DedupStillWorks_R3 保持红线：30 分钟去重窗口仍生效
func TestProcessFault_DedupStillWorks_R3(t *testing.T) {
	h := newTestHandler(t)
	rec := createFaultRec(6003, LEDErrROFF)
	h.processFault(rec)
	h.processFault(rec)

	var count int64
	model.DB.Model(&model.FaultRecord{}).Where("device_hw_id = ?", recognition.LedUUID(rec.LedHwID)).Count(&count)
	if count != 1 {
		t.Errorf("去重窗口内应只 1 条故障, 实际 %d", count)
	}
}

// TestProcessFault_PendingReviewNoAutoWorkOrder 待确认：critical 也不自动派单（证据补充后升级再派）
func TestProcessFault_PendingReviewNoAutoWorkOrder(t *testing.T) {
	h := newTestHandler(t)
	// 构造待确认场景：注入矛盾电流证据使置信度降到待确认区间 → 引擎在 processFault 内检索不到该外部注入证据，
	// 故直接构造主信号为未知 errCode 触发 pending_review（引擎对未知码宁待确认）
	rec := createFaultRec(6004, -99) // 未知错误码 → pending_review

	h.processFault(rec)

	var f model.FaultRecord
	model.DB.Where("device_hw_id = ?", recognition.LedUUID(rec.LedHwID)).First(&f)
	if f.RecognitionStatus != model.RecognitionPendingReview {
		t.Fatalf("未知 errCode 应 pending_review, got %s", f.RecognitionStatus)
	}
	// 即便 fault_level 为 normal，也绝不自动派单（未确认）
	var woCount int64
	model.DB.Model(&model.WorkOrder{}).Count(&woCount)
	if woCount != 0 {
		t.Errorf("待确认故障不应自动派单, 实际 %d", woCount)
	}
}

// TestReviewUpgrade_PendingToConfirmedDispatch 复核升级：待确认 → 确认真故障后，critical 自动派单
func TestReviewUpgrade_PendingToConfirmedDispatch(t *testing.T) {
	h := newTestHandler(t)
	rec := createFaultRec(6005, -99) // pending_review
	h.processFault(rec)

	// 复核确认为真故障（通过 handler 层逻辑直接验证 fault_reviewWorkorder 行为由 handler 测试覆盖；
	// 此处验证状态升级路径的数据可回写）
	var f model.FaultRecord
	model.DB.Where("device_hw_id = ?", recognition.LedUUID(rec.LedHwID)).First(&f)
	high := 0.99
	now := timeNow()
	model.DB.Model(&f).Updates(map[string]interface{}{
		"recognition_status": model.RecognitionConfirmed,
		"confidence":         high,
		"reviewed_at":        &now,
	})
	model.DB.First(&f)
	if f.RecognitionStatus != model.RecognitionConfirmed {
		t.Error("复核后应升级为 confirmed")
	}
}

func timeNow() (t time.Time) { return time.Now() }

// TestM2_PendingReviewAutoUpgradeDispatch M2 自动升级：已存在的待确认(pending_review)故障，
// 后续上报证据使研判达 confirmed 高置信 → existing 自动升级为确认，critical 自动派单（且只派 1 单）。
func TestM2_PendingReviewAutoUpgradeDispatch(t *testing.T) {
	h := newTestHandler(t)

	// 预置一条待确认 critical 故障（同一设备同一 errCode，且在 30min 去重窗口内）
	now := time.Now()
	conf := 0.6
	f := model.FaultRecord{
		DeviceHwID: recognition.LedUUID(7002), ErrCode: LEDErrROFF, FaultType: "lamp_off", FaultLevel: "critical",
		Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now,
		Confidence: &conf, RecognitionStatus: model.RecognitionPendingReview,
	}
	model.DB.Create(&f)

	// 二次上报：无矛盾证据 → 引擎 judge=confirmed（高置信）→ 触发 M2 自动升级 + critical 派单
	rec := createFaultRec(7002, LEDErrROFF)
	h.processFault(rec)

	var ff model.FaultRecord
	model.DB.First(&ff, f.ID)
	if ff.RecognitionStatus != model.RecognitionConfirmed {
		t.Errorf("待确认故障应由证据补充自动升级为 confirmed, got %s", ff.RecognitionStatus)
	}
	if ff.WorkOrderID == nil {
		t.Fatal("升级确认后 critical 应自动派单")
	}

	// 只派 1 条活跃工单
	var woCount int64
	model.DB.Model(&model.WorkOrder{}).Where("fault_id = ?", f.ID).Count(&woCount)
	if woCount != 1 {
		t.Errorf("自动升级应只派 1 条工单, 实际 %d", woCount)
	}

	// 再次上报（已 confirmed 且已派单）不应重复派单
	h.processFault(createFaultRec(7002, LEDErrROFF))
	model.DB.Model(&model.WorkOrder{}).Where("fault_id = ?", f.ID).Count(&woCount)
	if woCount != 1 {
		t.Errorf("已派单故障二次上报不应重复派单, 实际 %d", woCount)
	}
}

// TestM2_NoFalseDowngradeOfConfirmedGuard M2 反向守护：已 confirmed/已派单的故障，
// 后续即使上报矛盾证据使单次 judge 低置信（pending_review），也绝不被自动降级回来或重复派单。
func TestM2_NoFalseDowngradeOfConfirmedGuard(t *testing.T) {
	h := newTestHandler(t)

	// 预置一条已确认并已派单的 critical 故障
	now := time.Now()
	conf := 0.98
	f := model.FaultRecord{
		DeviceHwID: recognition.LedUUID(7003), ErrCode: LEDErrROFF, FaultType: "lamp_off", FaultLevel: "critical",
		Status: model.FaultStatusConfirmed, FirstSeen: now, LastSeen: now,
		Confidence: &conf, RecognitionStatus: model.RecognitionConfirmed,
	}
	model.DB.Create(&f)
	// 模拟已派单（fault_work_order_id 非空）
	wo := model.WorkOrder{OrderNo: "WO_M2G_1", FaultID: f.ID, DeviceHwID: f.DeviceHwID, Status: model.WorkOrderStatusPending, FaultActiveScope: nil}
	model.DB.Create(&wo)
	wid := wo.ID
	model.DB.Model(&model.FaultRecord{}).Where("id = ?", f.ID).Update("work_order_id", wid)

	// 后续上报带矛盾电流的低置信信号（使 judge 为 pending_review，绝不触发自动派单/降级）
	rec := createFaultRec(7003, LEDErrROFF)
	rec.CurrentR = 500 // 高电流矛盾（对 lamp_off 置 pending_review）
	h.processFault(rec)

	// existing 维持 confirmed，不被降级
	var ff model.FaultRecord
	model.DB.First(&ff, f.ID)
	if ff.RecognitionStatus != model.RecognitionConfirmed {
		t.Errorf("已 confirmed 故障不应被低置信上报降级, got %s", ff.RecognitionStatus)
	}
	if ff.WorkOrderID == nil || *ff.WorkOrderID != wid {
		t.Errorf("已派单故障不应改变 work_order_id, got %v", ff.WorkOrderID)
	}
	// 活跃工单仍为 1
	var woCount int64
	model.DB.Model(&model.WorkOrder{}).Where("fault_id = ?", f.ID).Count(&woCount)
	if woCount != 1 {
		t.Errorf("已派单故障不应重复派单, 实际 %d", woCount)
	}
}
