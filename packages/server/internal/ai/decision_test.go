package ai

import (
	"testing"
)

// TestLevelOf 评分等级映射
func TestLevelOf(t *testing.T) {
	if levelOf(85) != "good" {
		t.Error("85 应为 good")
	}
	if levelOf(70) != "warn" {
		t.Error("70 应为 warn")
	}
	if levelOf(50) != "bad" {
		t.Error("50 应为 bad")
	}
}

// TestClamp01 分数裁剪
func TestClamp01(t *testing.T) {
	if clamp01(120) != 100 {
		t.Error("120 应裁剪为 100")
	}
	if clamp01(-5) != 0 {
		t.Error("-5 应裁剪为 0")
	}
	if clamp01(66) != 66 {
		t.Error("66 应保持不变")
	}
}

// TestPrioRank 优先级排序
func TestPrioRank(t *testing.T) {
	if prioRank("high") >= prioRank("medium") {
		t.Error("high 应排在 medium 前")
	}
	if prioRank("medium") >= prioRank("low") {
		t.Error("medium 应排在 low 前")
	}
}

// TestRuleClassifyOpsHealth 规则识别健康评分（无 LLM/DB 依赖）
func TestRuleClassifyOpsHealth(t *testing.T) {
	it := ruleClassify("运维健康评分是多少")
	if it.Tool != "ops_health" {
		t.Errorf("运维健康评分 应识别为 ops_health, got %s", it.Tool)
	}
	it = ruleClassify("给出决策建议")
	if it.Tool != "ops_health" {
		t.Errorf("决策建议 应识别为 ops_health, got %s", it.Tool)
	}
	// 不应把普通查询误判
	it = ruleClassify("最近7天哪些路口故障最多")
	if it.Tool == "ops_health" {
		t.Error("正常故障排行不应误判为 ops_health")
	}
}
