package ai

import (
	"strings"
	"testing"
)

// 覆盖 helpers.go / 其他 ai 包纯工具函数。

func TestJsonFactors(t *testing.T) {
	// 空 → 默认文案
	got := jsonFactors(nil)
	if !strings.Contains(got, "暂无显著风险因子") {
		t.Errorf("空 factors 应含默认文案, got %q", got)
	}
	// 有因子 → JSON 数组
	got = jsonFactors([]string{"高发时段", "设备老化"})
	if !strings.Contains(got, "高发时段") || !strings.Contains(got, "设备老化") {
		t.Errorf("jsonFactors=%q", got)
	}
	// 单元素
	if got := jsonFactors([]string{"单一"}); got != `["单一"]` {
		t.Errorf("单元素=%q", got)
	}
}
