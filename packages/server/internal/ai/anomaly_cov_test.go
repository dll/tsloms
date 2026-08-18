package ai

import (
	"strings"
	"testing"

	"github.com/tsloms/server/internal/model"
)

// 覆盖 ai/anomaly.go 纯工具函数：classifyPacket / pktLevel / sortEventsDesc / ruleAnomalySummary。

func TestAnomaly_classifyPacket(t *testing.T) {
	// 告警命令 → packet_alarm
	kind, title, detail := classifyPacket(model.PacketLog{CmdType: 0x01})
	if kind != "packet_alarm" || !strings.Contains(title, "告警") || detail == "" {
		t.Errorf("alarm: kind=%q title=%q detail=%q", kind, title, detail)
	}
	// 签到命令 → 不异常
	kind, _, _ = classifyPacket(model.PacketLog{CmdType: 0x00})
	if kind != "" {
		t.Errorf("签到应不异常, got %q", kind)
	}
	// 未知命令 + 无效 → packet_invalid
	kind, title, _ = classifyPacket(model.PacketLog{CmdType: 0xEE, Valid: false})
	if kind != "packet_invalid" || !strings.Contains(title, "无效") {
		t.Errorf("无效报文: kind=%q title=%q", kind, title)
	}
	// 未知命令 + 有效 → 无 kind
	kind, _, _ = classifyPacket(model.PacketLog{CmdType: 0xEE, Valid: true})
	if kind != "" {
		t.Errorf("有效但未知命令应无 kind, got %q", kind)
	}
}

func TestAnomaly_pktLevel(t *testing.T) {
	cases := map[string]string{
		"packet_invalid": "major",
		"packet_alarm":   "critical",
		"whatever":       "minor",
		"":               "minor",
	}
	for in, want := range cases {
		if got := pktLevel(in); got != want {
			t.Errorf("pktLevel(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestAnomaly_sortEventsDesc(t *testing.T) {
	ev := []AnomalyEvent{
		{Time: "2026-08-19T03:00:00"},
		{Time: "2026-08-19T05:00:00"},
		{Time: "2026-08-19T01:00:00"},
	}
	sortEventsDesc(ev)
	if ev[0].Time != "2026-08-19T05:00:00" || ev[2].Time != "2026-08-19T01:00:00" {
		t.Errorf("排序后顺序错误: %v", ev)
	}
	// 空 slice
	sortEventsDesc(nil)
}

func TestAnomaly_ruleSummary(t *testing.T) {
	// 无事件
	if s := ruleAnomalySummary(&AnomalyStreamResult{Total: 0}); !strings.Contains(s, "无异常") {
		t.Errorf("零事件摘要=%q", s)
	}
	// critical + major
	res := &AnomalyStreamResult{
		Total:   3,
		ByLevel: map[string]int{"critical": 1, "major": 2},
	}
	s := ruleAnomalySummary(res)
	if !strings.Contains(s, "1 项严重异常") || !strings.Contains(s, "2 项重要异常") {
		t.Errorf("critical/major 摘要=%q", s)
	}
	// 只有 minor
	s = ruleAnomalySummary(&AnomalyStreamResult{Total: 1, ByLevel: map[string]int{"minor": 1}})
	_ = s
}
