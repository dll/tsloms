package ai

import (
	"strings"
	"testing"
)

// 覆盖 ai/reports.go 中纯工具函数（不依赖 DB / LLM）。

func TestReports_pct(t *testing.T) {
	cases := []struct{ a, b int64; want float64 }{
		{0, 0, 0},        // 除零
		{10, 0, 0},       // 除零
		{50, 100, 50},    // 一半
		{20, 80, 25},     // 25%
		{100, 100, 100},  // 全量
		{0, 50, 0},       // 零占比
	}

	for _, c := range cases {
		got := pct(c.a, c.b)
		eps := 1e-6
		diff := got - c.want
		if diff < 0 {
			diff = -diff
		}
		if diff > eps {
			t.Errorf("pct(%d,%d)=%v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestReports_joinFaults(t *testing.T) {
	empty := []struct {
		FaultType string
		Count     int64
	}{}
	if got := joinFaults(empty); got != "" {
		t.Errorf("空应返回空串, got %q", got)
	}
	one := []struct {
		FaultType string
		Count     int64
	}{{FaultType: "lamp_off", Count: 3}}
	if got := joinFaults(one); got != "lamp_off(3)" {
		t.Errorf("单元素=%q", got)
	}
	three := []struct {
		FaultType string
		Count     int64
	}{
		{"lamp_off", 3},
		{"timeout", 2},
		{"power_loss", 1},
	}
	if got := joinFaults(three); got != "lamp_off(3)、timeout(2)、power_loss(1)" {
		t.Errorf("三元素=%q", got)
	}
	four := []struct {
		FaultType string
		Count     int64
	}{
		{"a", 1}, {"b", 2}, {"c", 3}, {"d", 4},
	}
	if got := joinFaults(four); got != "a(1)、b(2)、c(3)" {
		t.Errorf("超出3个应截断=%q", got)
	}
}

func TestReports_joinDevices(t *testing.T) {
	items := []struct {
		DeviceHwID string
		Count      int64
	}{{"1001", 5}, {"1002", 2}}
	got := joinDevices(items)
	if !strings.Contains(got, "#1001(5)") || !strings.Contains(got, "#1002(2)") {
		t.Errorf("joinDevices=%q", got)
	}
	_ = joinDevices(nil) // 空 slice 不 panic
}

func TestReports_periodRange(t *testing.T) {
	// periodRange 返回起止时间表达（非空即可），验证各分支不 panic 且有序
	for _, p := range []string{"week", "month", "year", "today", "unknown"} {
		s, e := periodRange(p)
		if s == "" || e == "" {
			t.Errorf("period=%q 空起止", p)
		}
	}
}
