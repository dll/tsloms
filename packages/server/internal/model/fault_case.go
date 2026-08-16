package model

import "time"

// FaultCase 识别案例库表（范围A：智能多源故障识别研判引擎）
// 沉淀每次研判样本（输入证据特征 + 引擎判定 + 回标真值），用于长尾学习与 100% 识别率达标。
type FaultCase struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	FaultType       string     `json:"fault_type" gorm:"size:32;index;comment:标准故障类型"`
	FaultLevel      string     `json:"fault_level" gorm:"size:16;comment:标准故障等级"`
	DeviceHwID      uint32     `json:"device_hw_id" gorm:"index;comment:关联设备硬件ID"`
	InputSignature  string     `json:"input_signature" gorm:"size:128;comment:输入证据指纹(特征组合签名,用于检索召回)"`
	EvidenceSummary string     `json:"evidence_summary" gorm:"type:text;comment:证据摘要(text)"`
	ExpectedResult  string     `json:"expected_result" gorm:"size:32;comment:回标真值:真实故障类型/等级"`
	JudgedResult    string     `json:"judged_result" gorm:"size:32;comment:引擎原始判定(用于对比学习)"`
	JudgeConfidence *float64   `json:"judge_confidence" gorm:"comment:引擎判定置信度"`
	IsCorrect       *bool      `json:"is_correct" gorm:"comment:引擎判定是否与真值一致"` // 判定结果为误报过滤时，expected=normal 视为判定正确
	SourceEvaluationID string `json:"source_evaluation_id" gorm:"size:40;comment:来源研判批次(可回溯)"`
	Status          string     `json:"status" gorm:"size:16;default:seed;comment:状态(seed/confirmed/training/test)"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (FaultCase) TableName() string { return "fault_case" }

// 案例库状态常量
const (
	CaseStatusSeed      = "seed"      // 种子/自动沉淀
	CaseStatusConfirmed = "confirmed" // 人工回标确认
	CaseStatusTraining  = "training"  // 训练中
	CaseStatusTest      = "test"      // 测试集
)
