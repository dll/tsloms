package caselib

import (
	"testing"

	"github.com/tsloms/server/internal/model"
	"github.com/tsloms/server/internal/recognition"
)

// setup 返回内存 SQLite 的案例库记录器
func setup(t *testing.T) *CaseRecorder {
	t.Helper()
	model.InitTestDB()
	return NewCaseRecorder(model.DB)
}

func newEval() *recognition.Evaluator {
	// 直接使用 faultcode 常量；这里借 model 无约束，构造 lamp_off 研判
	return recognition.NewEvaluator(5001, -1, 0, 100, 100, 100)
}

// mustConfirmed 进行一次高置信研判（确认）
func mustConfirmed() (model.FaultRecognition, *recognition.Evaluator) {
	e := newEval()
	judge := e.Validate()
	return judge, e
}

// TestSeedRecord_PersistsCase 案例库写库：确认真故障 → 案例 expected=故障类型
func TestSeedRecord_PersistsCase(t *testing.T) {
	cr := setup(t)
	judge, e := mustConfirmed()
	cs, err := cr.SeedRecord(e, judge, judge.EvaluationID)
	if err != nil {
		t.Fatalf("seed err: %v", err)
	}
	if cs == nil {
		t.Fatal("案例不应为空")
	}
	if cs.ExpectedResult != "lamp_off" {
		t.Errorf("确认真故障 expected=%s, 期望 lamp_off", cs.ExpectedResult)
	}
	if cs.IsCorrect == nil || !*cs.IsCorrect {
		t.Error("判定应与真值一致(正确)")
	}
}

// TestSeedRecord_Dedup 案例去重：同特征同设备不重复写
func TestSeedRecord_Dedup(t *testing.T) {
	cr := setup(t)
	judge, e := mustConfirmed()
	_, _ = cr.SeedRecord(e, judge, judge.EvaluationID)
	_, _ = cr.SeedRecord(e, judge, judge.EvaluationID)

	var total int64
	model.DB.Model(&model.FaultCase{}).Where("device_hw_id = ?", recognition.LedUUID(5001)).Count(&total)
	if total != 1 {
		t.Errorf("同特征同设备应只写 1 条案例, 实际 %d", total)
	}
}

// TestSeedRecord_FilteredCase 误报过滤案例：expected 标为非故障(normal)
func TestSeedRecord_FilteredCase(t *testing.T) {
	cr := setup(t)
	// 构造一个被过滤的研判（未知错误码或明确否证）
	e := newEval()
	judge := model.FaultRecognition{
		FaultType:         "lamp_off",
		FaultLevel:        "critical",
		Confidence:        0.2,
		RecognitionStatus: model.RecognitionFiltered,
		RecognitionSource: model.RecognitionSourceMultiSource,
		EvidenceCount:     2,
		EvaluationID:      "ev-1",
	}
	cs, err := cr.SeedRecord(e, judge, judge.EvaluationID)
	if err != nil {
		t.Fatalf("seed err: %v", err)
	}
	if cs.ExpectedResult != "normal" || cs.JudgedResult != "normal" {
		t.Errorf("过滤案例 expected/judged=%s/%s, 期望 normal/normal", cs.ExpectedResult, cs.JudgedResult)
	}
	if cs.IsCorrect == nil || !*cs.IsCorrect {
		t.Error("过滤(判为非故障)应符合真值,应为正确")
	}
}

// TestTrain_Skeleton 训练骨架：确认真故障案例下返回正确率与 100% 达标
func TestTrain_Skeleton(t *testing.T) {
	cr := setup(t)
	judge, e := mustConfirmed()
	_, _ = cr.SeedRecord(e, judge, judge.EvaluationID)

	res, err := cr.Train()
	if err != nil {
		t.Fatalf("train err: %v", err)
	}
	total := res["total_cases"].(int64)
	if total != 1 {
		t.Errorf("total_cases=%d, 期望 1", total)
	}
	// 规则主通道对该案例 100% 命中 → recognize_100pct
	if res["recognize_100pct"] != true {
		t.Error("全部案例规则命中时应标记 100% 识别率达标")
	}
}

// TestStats 识别统计：确认真故障样本下误报/漏报为 0，识别率 100%
func TestStats(t *testing.T) {
	cr := setup(t)
	judge, e := mustConfirmed()
	_, _ = cr.SeedRecord(e, judge, judge.EvaluationID)

	res, err := cr.Stats()
	if err != nil {
		t.Fatalf("stats err: %v", err)
	}
	if res["total_cases"].(int64) != 1 {
		t.Errorf("total_cases=%v, 期望 1", res["total_cases"])
	}
	if res["false_positive"].(int64) != 0 || res["false_negative"].(int64) != 0 {
		t.Errorf("确认真故障下误报/漏报应为 0, got fp=%v fn=%v", res["false_positive"], res["false_negative"])
	}
	if res["accuracy"].(float64) != 1.0 {
		t.Errorf("确认样本识别率应为 1.0, got %v", res["accuracy"])
	}
}

// TestScoreByRules 规则打分骨架恒等于研判置信度
func TestScoreByRules(t *testing.T) {
	judge, _ := mustConfirmed()
	if ScoreByRules(judge) != judge.Confidence {
		t.Error("规则打分应等于研判置信度")
	}
}
