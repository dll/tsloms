package ai

import (
	"testing"

	"github.com/tsloms/server/internal/model"
)

// TestClassifyPacket 报文分类
func TestClassifyPacket(t *testing.T) {
	// 告警帧(0x01) → packet_alarm
	lvl, title, detail := classifyPacket(model.PacketLog{CmdType: 0x01, Valid: true})
	if lvl != "packet_alarm" {
		t.Errorf("告警帧 kind=%q, 期望 packet_alarm", lvl)
	}
	if title == "" || detail == "" {
		t.Errorf("告警帧 title/detail 不应为空: %q / %q", title, detail)
	}

	// 签到帧(0x00) → 非异常（跳过）
	lvl, _, _ = classifyPacket(model.PacketLog{CmdType: 0x00, Valid: true})
	if lvl != "" {
		t.Errorf("签到帧 kind=%q, 期望空(跳过)", lvl)
	}

	// 其他帧且无效 → packet_invalid
	lvl, _, _ = classifyPacket(model.PacketLog{CmdType: 0x30, Valid: false})
	if lvl != "packet_invalid" {
		t.Errorf("无效帧 kind=%q, 期望 packet_invalid", lvl)
	}

	// 其他帧且有效 → 非异常
	lvl, _, _ = classifyPacket(model.PacketLog{CmdType: 0x30, Valid: true})
	if lvl != "" {
		t.Errorf("有效其他帧 kind=%q, 期望空", lvl)
	}
}

// TestSortEventsDesc 时间倒序排序
func TestSortEventsDesc(t *testing.T) {
	ev := []AnomalyEvent{
		{Time: "2026-08-15T10:00:00+08:00", Title: "a"},
		{Time: "2026-08-15T12:00:00+08:00", Title: "c"},
		{Time: "2026-08-15T11:00:00+08:00", Title: "b"},
	}
	sortEventsDesc(ev)
	if ev[0].Title != "c" || ev[1].Title != "b" || ev[2].Title != "a" {
		t.Errorf("排序错误: %v %v %v", ev[0].Title, ev[1].Title, ev[2].Title)
	}
}

// TestRuleAnomalySummary 规则摘要
func TestRuleAnomalySummary(t *testing.T) {
	// 空事件
	res := &AnomalyStreamResult{Events: []AnomalyEvent{}, Total: 0, ByLevel: map[string]int{}}
	if s := ruleAnomalySummary(res); s != "最近无异常事件，系统运行平稳。" {
		t.Errorf("空事件摘要=%q", s)
	}
	// 有严重异常
	res2 := &AnomalyStreamResult{Total: 5, ByLevel: map[string]int{"critical": 2, "major": 3}}
	s2 := ruleAnomalySummary(res2)
	if s2 == "" || !strContains(s2, "2 项严重异常") || !strContains(s2, "3 项重要异常") {
		t.Errorf("异常摘要=%q", s2)
	}
}

func strContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestNlRequirePermNoDB 无数据库时写命令应被拒绝而非 panic
func TestNlRequirePermNoDB(t *testing.T) {
	deny, ans := nlRequirePerm(1, "workorder:create")
	if !deny {
		t.Error("无数据库时应拒绝写命令")
	}
	if ans.Reply == "" {
		t.Error("应返回明确提示")
	}
}
