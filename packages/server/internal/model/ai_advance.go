package model

import "time"

// ===== AI 原生增强：报告持久化 =====

// AIReport AI 生成的运维报告（各模块分析/报告均持久化，支持历史对比）
type AIReport struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Module     string    `json:"module" gorm:"size:24;index;comment:模块(daily/inventory/cost/fault/workorder/device/advice)"`
	Title      string    `json:"title" gorm:"size:128;comment:报告标题"`
	Period     string    `json:"period" gorm:"size:16;comment:周期类型(day/week/month/range)"`
	RangeFrom  string    `json:"range_from" gorm:"size:16;comment:统计起始(2026-08-01)"`
	RangeTo    string    `json:"range_to" gorm:"size:16;comment:统计截止"`
	Summary    string    `json:"summary" gorm:"type:text;comment:LLM/规则生成的人工可读摘要"`
	Data       string    `json:"data" gorm:"type:mediumtext;comment:结构化统计快照(JSON)"`
	Insights   string    `json:"insights" gorm:"type:text;comment:结论/建议(JSON数组)"`
	Source     string    `json:"source" gorm:"size:16;comment:LLM/规则"`
	TokensUsed int       `json:"tokens_used" gorm:"default:0;comment:消耗token"`
	Operator   string    `json:"operator" gorm:"size:64;comment:生成人"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName
func (AIReport) TableName() string { return "ai_reports" }

// ===== AI 原生增强：核心流程建议（故障/工单 Copilot） =====

// AIAdvice AI 核心流程建议记录（故障确认/派单、工单处理时的 AI 建议）
type AIAdvice struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	BizType    string    `json:"biz_type" gorm:"size:24;index;comment:业务类型(fault/workorder)"`
	BizID      uint      `json:"biz_id" gorm:"index;comment:关联业务ID"`
	DeviceHwID uint32    `json:"device_hw_id" gorm:"index;comment:设备硬件ID"`
	Stage      string    `json:"stage" gorm:"size:24;comment:阶段(diagnose/dispatch/copilot/summary)"`
	Priority   string    `json:"priority" gorm:"size:16;comment:建议优先级(P0/P1/P2)"`
	Summary    string    `json:"summary" gorm:"type:text;comment:AI摘要/根因预判"`
	Action     string    `json:"action" gorm:"type:text;comment:建议动作列表(JSON)"`
	Parts      string    `json:"parts" gorm:"type:text;comment:建议备件列表(JSON)"`
	Plan       string    `json:"plan" gorm:"type:text;comment:应对预案/处理步骤"`
	Content    string    `json:"content" gorm:"type:text;comment:完整生成文本"`
	Source     string    `json:"source" gorm:"size:16;comment:LLM/规则"`
	TokensUsed int       `json:"tokens_used" gorm:"default:0"`
	Operator   string    `json:"operator" gorm:"size:64;comment:触发人"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName
func (AIAdvice) TableName() string { return "ai_advices" }
