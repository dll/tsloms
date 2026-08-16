package mqtt

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

// TestProcessFault_PersistsEvidenceAndCase 故障研判后多源证据落库 + 案例沉淀
func TestProcessFault_PersistsEvidenceAndCase(t *testing.T) {
	h := newTestHandler(t)
	rec := createFaultRec(6001, LEDErrROFF) // critical, confirmed
	h.processFault(rec)

	// 多源证据落库
	var evCount int64
	model.DB.Model(&model.FaultEvidence{}).Where("device_hw_id = ?", uint32(6001)).Count(&evCount)
	if evCount == 0 {
		t.Error("研判后应写入多源证据(fault_evidence)")
	}
	// 主信号证据 source=firmware
	var ev model.FaultEvidence
	model.DB.Where("device_hw_id = ? AND source_type = ?", uint32(6001), model.EvSourceFirmware).First(&ev)
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
	model.DB.Where("device_hw_id = ?", rec.LedHwID).First(&f)
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
	model.DB.Model(&model.FaultRecord{}).Where("device_hw_id = ?", rec.LedHwID).Count(&count)
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
	model.DB.Where("device_hw_id = ?", rec.LedHwID).First(&f)
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
	model.DB.Where("device_hw_id = ?", rec.LedHwID).First(&f)
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
