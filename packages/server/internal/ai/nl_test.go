package ai

import (
	"testing"
)

// L5 自然语言交互测试：规则识别 + 工具执行（无 LLM/无 DB 的纯函数部分）。

func TestRuleClassify(t *testing.T) {
	cases := []struct {
		in   string
		tool string
	}{
		{"最近7天哪些路口故障最多", "fault_rank"},
		{"最近一周哪个路口故障最多", "fault_rank"},
		{"查询设备123456状态", "device_status"},
		{"设备 888 在线吗", "device_status"},
		{"工作单统计一下", "workorder_stats"},
		{"最近30天维修费用多少", "expense_summary"},
		{"报修：人民路口红灯不亮", "create_fault"},
		{"给设备123456建工单", "create_workorder"},
		{"怎么新建工单？", "kb"},
	}
	for _, c := range cases {
		it := ruleClassify(c.in)
		if it.Tool != c.tool {
			t.Errorf("ruleClassify(%q) tool = %q, want %q", c.in, it.Tool, c.tool)
		}
	}
}

func TestExtractDays(t *testing.T) {
	if got := extractDays("最近7天"); got != "7" {
		t.Errorf("extractDays(7天) = %q, want 7", got)
	}
	if got := extractDays("最近一周"); got != "7" {
		t.Errorf("extractDays(一周) = %q, want 7", got)
	}
	if got := extractDays("最近30天费用"); got != "30" {
		t.Errorf("extractDays(30天) = %q, want 30", got)
	}
}

func TestParseHwID(t *testing.T) {
	if got := parseHwID("123456"); got != 123456 {
		t.Errorf("parseHwID(123456) = %d", got)
	}
	if got := parseHwID("abc"); got != 0 {
		t.Errorf("parseHwID(abc) = %d, want 0", got)
	}
}

func TestExtractHwID(t *testing.T) {
	if got := extractHwID("报修：设备1黄灯不亮"); got != "1" {
		t.Errorf("extractHwID(设备1) = %q, want 1", got)
	}
	if got := extractHwID("查询设备123456状态"); got != "123456" {
		t.Errorf("extractHwID(设备123456) = %q, want 123456", got)
	}
}

func TestExtractIntersection(t *testing.T) {
	if got := extractIntersection("人民路与建设路交叉口黄灯不亮"); got != "人民路与建设路交叉口" {
		t.Errorf("extractIntersection(交叉口) = %q", got)
	}
	if got := extractIntersection("报修：人民路路口红灯不亮"); got != "人民路路口" {
		t.Errorf("extractIntersection(带冒号) = %q", got)
	}
}

func TestCleanDescAndType(t *testing.T) {
	if got := cleanDesc("报修：人民路黄灯不亮"); got != "人民路黄灯不亮" {
		t.Errorf("cleanDesc = %q", got)
	}
	if got := detectFaultType("绿灯闪烁"); got != "绿灯故障" {
		t.Errorf("detectFaultType = %q", got)
	}
	if got := detectErrCode("红灯不亮"); got != -1 {
		t.Errorf("detectErrCode 红灯 = %d, want -1", got)
	}
}

func TestMapLevel(t *testing.T) {
	if mapLevel("严重的全灭故障") != "critical" {
		t.Error("严重应映射 critical")
	}
	if mapLevel("轻微黄灯问题") != "normal" {
		t.Error("轻微应映射 normal")
	}
}

func TestIsHowQuestion(t *testing.T) {
	if !isHowQuestion("怎么新建工单？") {
		t.Error("怎么新建工单应为操作咨询")
	}
	if !isHowQuestion("如何查询设备状态") {
		t.Error("如何查询应为操作咨询")
	}
	if isHowQuestion("最近7天哪些路口故障最多") {
		t.Error("查询不应误判为咨询")
	}
	if isHowQuestion("报修：人民路红灯不亮") {
		t.Error("报修不应误判为咨询")
	}
}

// 空输入兜底（不会触达 LLM/DB，安全）
func TestInterpretNLEmpty(t *testing.T) {
	ans := InterpretNL(0, "")
	if ans.Reply == "" {
		t.Error("空输入应返回提示")
	}
	if ans.Intent != "fallback" {
		t.Errorf("空输入 intent = %q, want fallback", ans.Intent)
	}
}
