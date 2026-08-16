package recognition

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/faultcode"
	"github.com/tsloms/server/internal/model"
)

// newEval 构造主信号为 lamp_off(err -1) 的研判上下文
func newEval() *Evaluator {
	return NewEvaluator(1001, faultcode.LEDErrROFF, faultcode.StateR, 100, 100, 100)
}

// TestValidate_HighConfidenceConfirmed 确定性规则主通道：红外 errCode 基础置信度足够高 → confirmed
func TestValidate_HighConfidenceConfirmed(t *testing.T) {
	e := newEval()
	judge := e.Validate()
	if judge.RecognitionStatus != model.RecognitionConfirmed {
		t.Fatalf("errCode ROFF 无异证应 confirmed, got %s (conf=%v)", judge.RecognitionStatus, judge.Confidence)
	}
	if judge.FaultType != "lamp_off" || judge.FaultLevel != "critical" {
		t.Errorf("类型/等级 = %s/%s, 期望 lamp_off/critical", judge.FaultType, judge.FaultLevel)
	}
	if judge.Confidence < ConfHigh {
		t.Errorf("基础置信度 %v 应 ≥ %v", judge.Confidence, ConfHigh)
	}
}

// TestValidate_ConfidenceBoostsByAuxEvidence 多源证据印证提升置信度
func TestValidate_ConfidenceBoostsByAuxEvidence(t *testing.T) {
	e := newEval()
	base := e.Validate().Confidence

	e2 := newEval()
	e2.AddEvidence(RuleEvidence{DeviceHwID: 1001, SourceType: model.EvSourceCitizen, RawData: "群众反映红灯长灭", CapturedAt: time.Now()})
	e2.AddEvidence(RuleEvidence{DeviceHwID: 1001, SourceType: model.EvSourcePhotoEvidence, RawData: "手机举证照片", CapturedAt: time.Now()})
	boosted := e2.Validate().Confidence
	if boosted <= base {
		t.Errorf("多源印证置信度 %v 应高于孤证 %v", boosted, base)
	}
}

// TestValidate_ConflictingCurrentDowngradesToPending 电流矛盾 → 显著降级 → 待确认（不直接判真/彻底过滤）
func TestValidate_ConflictingCurrentDowngradesToPending(t *testing.T) {
	e := newEval()
	// lamp_off(err -1) 但红灯电流正常高 → 明确否证，显著降级
	r := uint16(500)
	e.AddEvidence(RuleEvidence{DeviceHwID: 1001, SourceType: model.EvSourceCurrent, CurrentR: &r})
	judge := e.Validate()
	if judge.RecognitionStatus != model.RecognitionPendingReview {
		t.Errorf("电流矛盾应降级待确认, got %s (conf=%v)", judge.RecognitionStatus, judge.Confidence)
	}
	if judge.RecognitionStatus == model.RecognitionFiltered {
		t.Error("电流矛盾绝不直接误报过滤（不漏真故障），应待确认")
	}
}

// TestValidate_CorroboratingCurrent 电流印证 lamp_off（红灯电流很低）→ 置信度提升且保持 confirmed
func TestValidate_CorroboratingCurrent(t *testing.T) {
	e := newEval()
	r := uint16(10)
	e.AddEvidence(RuleEvidence{DeviceHwID: 1001, SourceType: model.EvSourceCurrent, CurrentR: &r})
	judge := e.Validate()
	if judge.RecognitionStatus != model.RecognitionConfirmed {
		t.Errorf("电流印证应保持 confirmed, got %s", judge.RecognitionStatus)
	}
}

// TestValidate_UnknownErrCodePending 未知错误码（缺规则映射）→ 宁待确认不高判/不误报
func TestValidate_UnknownErrCodePending(t *testing.T) {
	e := NewEvaluator(2002, -99, faultcode.StateR, 100, 100, 100)
	judge := e.Validate()
	if judge.RecognitionStatus != model.RecognitionPendingReview {
		t.Errorf("未知 errCode 应 pending_review, got %s", judge.RecognitionStatus)
	}
}

// TestValidate_NormalNoAuxStaysConfirmed 一般故障（timeout）无异证仍符合规则 → confirmed
func TestValidate_NormalNoAuxStaysConfirmed(t *testing.T) {
	e := NewEvaluator(3003, faultcode.LEDErrGONTimeout, faultcode.StateG, 100, 100, 100)
	judge := e.Validate()
	if judge.RecognitionStatus != model.RecognitionConfirmed {
		t.Errorf("timeout 一般故障应 confirmed, got %s", judge.RecognitionStatus)
	}
	if judge.FaultLevel != "normal" {
		t.Errorf("timeout 应为 normal 等级, got %s", judge.FaultLevel)
	}
}

// TestBuildSignature 证据指纹稳定且包含主信号+辅助来源
func TestBuildSignature(t *testing.T) {
	e := newEval()
	e.AddEvidence(RuleEvidence{SourceType: model.EvSourceCitizen})
	sig := BuildSignature(e)
	if sig == "" {
		t.Fatal("签名不应为空")
	}
	// 同一主信号但辅助来源不同 → 签名不同（用于案例区分）
	e2 := newEval()
	if BuildSignature(e) == BuildSignature(e2) {
		t.Error("含/不含辅助证据的签名应不同")
	}
}

// TestValidate_SafetyNeverFilterRealFault 安全红线：已知 errCode（真实故障信号）在任意证据组合下
// 绝不被误报过滤（filtered），至多降级为待确认（宁等证据不漏真故障），且不崩溃。
// 该用例锁定 pm-checklist S4「误报过滤不漏真故障」的最高优先级约束。
func TestValidate_SafetyNeverFilterRealFault(t *testing.T) {
	// 全量已知 errCode × 破坏性最强的矛盾证据组合：三通道电流矛盾(-0.30)
	known := []int8{
		faultcode.LEDErrROFF, faultcode.LEDErrYOFF, faultcode.LEDErrGOFF,
		faultcode.LEDErrRYON, faultcode.LEDErrRGON, faultcode.LEDErrYGON, faultcode.LEDErrRYGON,
		faultcode.LEDErrRONTimeout, faultcode.LEDErrYONTimeout, faultcode.LEDErrGONTimeout,
		faultcode.LEDErrRDim, faultcode.LEDErrYDim, faultcode.LEDErrGDim,
		faultcode.LEDErrPowerLoss,
	}
	for _, ec := range known {
		e := NewEvaluator(9999, ec, faultcode.StateR, 100, 100, 100)
		// 注入三通道高电流（明确矛盾，关联灯色电流高 → 否证）
		r, y, g := uint16(500), uint16(500), uint16(500)
		e.AddEvidence(RuleEvidence{DeviceHwID: 9999, SourceType: model.EvSourceCurrent, CurrentR: &r, CurrentY: &y, CurrentG: &g})

		judge := e.Validate()
		if judge.RecognitionStatus == model.RecognitionFiltered {
			t.Errorf("已知 errCode(%d) 在矛盾证据下被误报过滤 → 违反不漏真故障红线", ec)
		}
		t.Logf("errCode %d 矛盾证据 → %s (conf=%v)", ec, judge.RecognitionStatus, judge.Confidence)
	}
}

// TestValidate_PartialChannelPowerLossPanic power_loss 单电流通道不得 panic：
// 文档允许“其它通道可不提供”，engine 必须对缺失通道安全取值（缺陷已修复，本用例守护不回归）。
func TestValidate_PartialChannelPowerLossPanic(t *testing.T) {
	e := NewEvaluator(8888, faultcode.LEDErrPowerLoss, faultcode.StateNone, 100, 100, 100)
	// 只提供红灯通道电流（文档声称“其它通道可不提供”），其余通道 nil；不得 panic
	r := uint16(500)
	e.AddEvidence(RuleEvidence{DeviceHwID: 8888, SourceType: model.EvSourceCurrent, CurrentR: &r})
	judge := e.Validate()
	if judge.Confidence <= 0 {
		t.Errorf("单通道 power_loss 电流不应 panic 且应产出有效置信度, got conf=%v", judge.Confidence)
	}
}

// TestValidate_FilteredOnlyViaManualReview 明确：引擎自动分流不会产生 filtered（需人工复核标记误报），
// 保证自动链路无需承担误报过滤风险；filtered 仅由 review 接口 created.Confirmed=false 落定。
func TestValidate_FilteredOnlyViaManualReview(t *testing.T) {
	// 最极端：基础置信度最低的已知码全灭类 + 最强矛盾电流证据，仍不落到 filtered
	e := newEval() // LEDErrROFF base 0.98
	r := uint16(500)
	e.AddEvidence(RuleEvidence{DeviceHwID: 1001, SourceType: model.EvSourceCurrent, CurrentR: &r})
	judge := e.Validate()
	if judge.RecognitionStatus == model.RecognitionFiltered {
		t.Fatal("自动分流不应误报过滤真实故障")
	}
	if judge.RecognitionStatus != model.RecognitionPendingReview {
		t.Errorf("矛盾证据应降到待确认, got %s (conf=%v)", judge.RecognitionStatus, judge.Confidence)
	}
}
