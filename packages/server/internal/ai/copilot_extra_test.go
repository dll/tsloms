package ai

import (
	"strings"
	"testing"
)

// 采购 Copilot 规则兜底：合法明细无校验告警 + 总额正确
func TestSuggestPurchaseCopilotValid(t *testing.T) {
	adv := SuggestPurchaseCopilot(0, []PurchaseLine{
		{MaterialName: "信号灯珠", Quantity: 10, Price: 12.5},
		{MaterialName: "控制器", Quantity: 2, Price: 300},
	}, 0)
	if adv.Source == "" {
		t.Fatal("source 不应为空")
	}
	if len(adv.Checks) != 0 {
		t.Fatalf("合法明细不应有校验告警, got %v", adv.Checks)
	}
	if !strings.Contains(adv.Summary, "合计") {
		t.Fatalf("summary 应含合计, got %q", adv.Summary)
	}
	if !strings.Contains(adv.SupplierHint, "尚未选择供应商") {
		t.Fatalf("未选供应商应有提示, got %q", adv.SupplierHint)
	}
}

// 采购 Copilot 规则兜底：数量<=0 / 负价 触发校验
func TestSuggestPurchaseCopilotInvalid(t *testing.T) {
	adv := SuggestPurchaseCopilot(0, []PurchaseLine{
		{MaterialName: "坏件", Quantity: 0, Price: 10},
		{MaterialName: "负价件", Quantity: 1, Price: -5},
	}, 0)
	if len(adv.Checks) < 2 {
		t.Fatalf("应至少触发2条校验, got %d: %v", len(adv.Checks), adv.Checks)
	}
}

// 设备 Copilot：hw_id 为空时给出必填校验
func TestSuggestDeviceCopilotEmptyHW(t *testing.T) {
	adv, err := SuggestDeviceCopilot(0, map[string]any{})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(adv.Summary, "hw_id") && !strings.Contains(adv.Summary, "硬件") {
		t.Fatalf("空 hw_id 应提示必填, got %q", adv.Summary)
	}
	if len(adv.Issues) == 0 {
		t.Fatalf("应存在 issue 提醒")
	}
}
