package model

import (
	"testing"
)

// ---- P0-3 预警规则匹配 ----

func TestWarningRuleMatches_Basic(t *testing.T) {
	rule := &WarningRule{
		Name:          "test",
		EffectiveType: RuleEffectivePermanent,
		Action:        RuleActionIgnore,
		Enabled:       true,
	}
	w := &Warning{WarningCode: -1, Level: WarningLevelCritical}

	if !rule.Matches(w) {
		t.Error("permanent ignore rule 应匹配任意预警")
	}
}

func TestWarningRuleMatches_CodeNotMatch(t *testing.T) {
	code := -5
	rule := &WarningRule{
		EffectiveType: RuleEffectivePermanent,
		Action:        RuleActionIgnore,
		Enabled:       true,
		WarningCode:   &code,
	}
	// 不同告警码 → 不匹配
	if rule.Matches(&Warning{WarningCode: -1}) {
		t.Error("告警码不同的预警不应匹配")
	}
	// 相同告警码 → 匹配
	if !rule.Matches(&Warning{WarningCode: -5}) {
		t.Error("告警码相同应匹配")
	}
}

func TestWarningRuleMatches_DeviceHw(t *testing.T) {
	hw := "42"
	rule := &WarningRule{
		EffectiveType: RuleEffectivePermanent,
		Action:        RuleActionIgnore,
		Enabled:       true,
		DeviceHwID:    &hw,
	}
	if rule.Matches(&Warning{DeviceHwID: "41"}) {
		t.Error("设备不同不应匹配")
	}
	if !rule.Matches(&Warning{DeviceHwID: "42"}) {
		t.Error("设备相同应匹配")
	}
}

func TestWarningRuleMatches_Disabled(t *testing.T) {
	rule := &WarningRule{EffectiveType: RuleEffectivePermanent, Action: RuleActionIgnore, Enabled: false}
	if rule.Matches(&Warning{}) {
		t.Error("停用规则不应匹配")
	}
}

func TestWarningRuleMatches_LevelFilter(t *testing.T) {
	rule := &WarningRule{EffectiveType: RuleEffectivePermanent, Action: RuleActionIgnore, Enabled: true, Level: WarningLevelCritical}
	if rule.Matches(&Warning{Level: WarningLevelInfo}) {
		t.Error("级别不同的预警不应匹配")
	}
	if !rule.Matches(&Warning{Level: WarningLevelCritical}) {
		t.Error("级别相同应匹配")
	}
}

func TestWarningRuleMatches_Crossing(t *testing.T) {
	cid := uint(7)
	rule := &WarningRule{EffectiveType: RuleEffectivePermanent, Action: RuleActionIgnore, Enabled: true, CrossingID: &cid}
	if rule.Matches(&Warning{CrossingID: nil}) {
		t.Error("规则限定路口但预警无路口，不应匹配")
	}
	if rule.Matches(&Warning{CrossingID: uintPtr(8)}) {
		t.Error("路口不同不应匹配")
	}
	if !rule.Matches(&Warning{CrossingID: uintPtr(7)}) {
		t.Error("路口相同应匹配")
	}
}

// ---- P0-5 路口聚合状态派生 ----

func TestComputeCrossingStatus(t *testing.T) {
	cases := []struct {
		fault, green float64
		want         string
	}{
		{0, 1, CrossingStatusNormal},       // 全绿
		{0, 0.6, CrossingStatusNormal},     // 无故障
		{0.3, 0.7, CrossingStatusAbnormal}, // 有故障渐变
		{1, 0, CrossingStatusOffline},      // 全部故障/断电
		{0.8, 0.2, CrossingStatusAbnormal},
	}
	for _, c := range cases {
		got := ComputeCrossingStatus(c.fault, c.green)
		if got != c.want {
			t.Errorf("ComputeCrossingStatus(%.1f, %.1f) = %s, want %s", c.fault, c.green, got, c.want)
		}
	}
}

// ---- P0-3 预警转工单活跃唯一（占位 fault=0 不破坏既有）----

func TestCreateStandaloneSeverity_NoConflict(t *testing.T) {
	db := InitTestDB()
	// 多条无来源故障预警工单（fault_id=0）应可并存（占位，不参与 fault 活跃唯一）
	wo1 := &WorkOrder{OrderNo: "WO_SA_1", FaultID: 0, DeviceHwID: "1", Status: WorkOrderStatusPending, FaultActiveScope: nil}
	wo2 := &WorkOrder{OrderNo: "WO_SA_2", FaultID: 0, DeviceHwID: "1", Status: WorkOrderStatusPending, FaultActiveScope: nil}
	if err := db.Create(wo1).Error; err != nil {
		t.Fatalf("创建占位工单1失败: %v", err)
	}
	if err := db.Create(wo2).Error; err != nil {
		t.Fatalf("创建占位工单2失败（占位 fault=0 不应触发活跃唯一冲突）: %v", err)
	}
}

func uintPtr(v uint) *uint { return &v }
