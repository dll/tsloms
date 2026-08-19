package ai

import "testing"

func TestCoverageFieldHelpersAndParsing(t *testing.T) {
	if fieldStr(map[string]any{"x": "ok"}, "x") != "ok" || fieldStr(map[string]any{"x": 1}, "x") != "" {
		t.Error("fieldStr 分支错误")
	}
	if fieldInt(map[string]any{"x": float64(3)}, "x") != 3 || fieldInt(map[string]any{"x": 4}, "x") != 4 || fieldInt(map[string]any{"x": "4"}, "x") != 0 {
		t.Error("fieldInt 分支错误")
	}
	for _, tc := range []struct {
		m    map[string]any
		want uint32
		ok   bool
	}{
		{map[string]any{"x": float64(3)}, 3, true},
		{map[string]any{"x": 4}, 4, true},
		{map[string]any{"x": "5"}, 5, true},
		{map[string]any{"x": "bad"}, 0, false},
	} {
		got, ok := fieldUint32(tc.m, "x")
		if got != tc.want || ok != tc.ok {
			t.Errorf("fieldUint32(%v)=(%d,%v)", tc.m, got, ok)
		}
	}
	if v, ok := fieldFloat(map[string]any{"x": 1.5}, "x"); !ok || v != 1.5 {
		t.Error("fieldFloat 正常分支错误")
	}
	if _, ok := fieldFloat(map[string]any{"x": "1.5"}, "x"); ok {
		t.Error("fieldFloat 类型错误分支错误")
	}
	if ifStr(true, "a", "b") != "a" || ifStr(false, "a", "b") != "b" {
		t.Error("ifStr 分支错误")
	}
	if firstLine("\n  第一行  \n第二行") != "第一行" || firstLine("  ") != "  " {
		t.Error("firstLine 分支错误")
	}
	if stripJSONFence("```json\n{\"a\":1}\n```") != "{\"a\":1}" || stripJSONFence("plain") != "plain" {
		t.Error("stripJSONFence 分支错误")
	}
	for _, tc := range []struct {
		in, want string
	}{
		{"material", "耗材"}, {"labor", "人工"}, {"traffic", "交通"}, {"other", "其他"}, {"unknown", "unknown"},
	} {
		if got := typeText(tc.in); got != tc.want {
			t.Errorf("typeText(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
	for _, tc := range []struct {
		in, want string
	}{
		{"红灯不亮", "红灯故障"}, {"黄灯异常", "黄灯故障"}, {"绿灯闪烁", "绿灯故障"}, {"灯组不亮", "灯组不亮"}, {"灯组闪烁", "灯组闪烁"}, {"断电", "线路故障"}, {"供电异常", "供电故障"}, {"正常", "自然语言报修"},
	} {
		if got := detectFaultType(tc.in); got != tc.want {
			t.Errorf("detectFaultType(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}
