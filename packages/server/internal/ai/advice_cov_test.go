package ai

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

func adviceSeed(t *testing.T) (uint, uint) {
	t.Helper()
	model.InitTestDB()
	op := model.User{Username: "op_adv", PasswordHash: "x", Role: model.RoleOperator}
	model.DB.Create(&op)
	model.DB.Create(&model.Device{HwID: 4001, Intersection: "建议路口", OnlineStatus: true})
	now := time.Now()
	f := model.FaultRecord{DeviceHwID: 4001, ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical",
		Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now, CurrentR: 300}
	model.DB.Create(&f)
	// 关联工单 + 历史领料
	wo := model.WorkOrder{OrderNo: "WOadv", FaultID: f.ID, DeviceHwID: 4001, Status: model.WorkOrderStatusProcessing, Result: "更换灯珠"}
	model.DB.Create(&wo)
	woID := wo.ID
	model.DB.Create(&model.MaterialStock{MaterialID: 1, MaterialName: "红灯灯珠", WorkOrderID: &woID, Type: model.StockTypeUse, Quantity: -1})
	return f.ID, wo.ID
}

func TestSuggestFaultAdvice_Rule(t *testing.T) {
	fid, _ := adviceSeed(t)
	adv, err := SuggestFaultAdvice(1, fid)
	if err != nil {
		t.Fatalf("SuggestFaultAdvice err: %v", err)
	}
	if adv.FaultID != fid {
		t.Errorf("FaultID=%d", adv.FaultID)
	}
	if adv.Source != "规则" {
		t.Errorf("无LLM时应走规则兜底, source=%s", adv.Source)
	}
	if adv.Priority != "P0" {
		t.Errorf("critical 故障优先级应为 P0, got %s", adv.Priority)
	}
	// 不存在的故障
	if _, err := SuggestFaultAdvice(1, 99999); err == nil {
		t.Error("不存在故障应报错")
	}
}

func TestSuggestWorkOrderAdvice_CopilotAndSummary(t *testing.T) {
	_, wid := adviceSeed(t)
	// copilot
	adv, err := SuggestWorkOrderAdvice(1, wid, "copilot")
	if err != nil {
		t.Fatalf("copilot err: %v", err)
	}
	if adv.Source != "规则" {
		t.Errorf("copilot 规则兜底 source=%s", adv.Source)
	}
	if len(adv.Parts) == 0 {
		t.Errorf("copilot 应有备件预领, parts=%v", adv.Parts)
	}
	// summary
	adv2, err := SuggestWorkOrderAdvice(1, wid, "summary")
	if err != nil {
		t.Fatalf("summary err: %v", err)
	}
	if adv2.Summary == "" {
		t.Error("summary 应有小结")
	}
	// stage 默认 copilot
	adv3, err := SuggestWorkOrderAdvice(1, wid, "")
	if err != nil {
		t.Fatalf("默认stage err: %v", err)
	}
	if adv3.WorkOrderID != wid {
		t.Errorf("WorkOrderID=%d", adv3.WorkOrderID)
	}
	// 不存在工单
	if _, err := SuggestWorkOrderAdvice(1, 99999, "copilot"); err == nil {
		t.Error("不存在工单应报错")
	}
}

func TestAdviceHelpers(t *testing.T) {
	// buildFaultRulePlan
	f := &model.FaultRecord{ErrCode: -1, FaultType: "lamp_off"}
	if p := buildFaultRulePlan(f); p == "" {
		t.Error("buildFaultRulePlan 空")
	}
	// buildCopilotRule（无故障/无备件分支）
	wo := &model.WorkOrder{OrderNo: "W1", DeviceHwID: 1}
	if p := buildCopilotRule(wo, nil, nil); p == "" {
		t.Error("buildCopilotRule 空")
	}
	// buildSummaryRule（有结果/无结果/无领料分支）
	if p := buildSummaryRule(wo, nil, nil); p == "" {
		t.Error("buildSummaryRule 空")
	}
	wo2 := &model.WorkOrder{OrderNo: "W2", Result: "完成"}
	used := []struct {
		MaterialName string
		Qty          int
	}{{"灯珠", 2}}
	if p := buildSummaryRule(wo2, &model.FaultRecord{ErrCode: -5, FaultType: "x"}, used); p == "" {
		t.Error("buildSummaryRule 空(含结果)")
	}
	// mapPriorityFault / mapStagePriority / priorityText
	if mapPriorityFault("critical") != "P0" || mapPriorityFault("major") != "P1" {
		t.Error("mapPriorityFault 错误")
	}
	if mapStagePriority("summary") != "P3" || mapStagePriority("copilot") != "P1" || mapStagePriority("other") != "P2" {
		t.Error("mapStagePriority 错误")
	}
	if priorityText("P0") == "" || priorityText("P3") == "" {
		t.Error("priorityText 空")
	}
	// deviceBrief
	model.InitTestDB()
	model.DB.Create(&model.Device{HwID: 77, Intersection: "某路口"})
	if deviceBrief(77) != "某路口" {
		t.Errorf("deviceBrief=%q", deviceBrief(77))
	}
	if deviceBrief(999) != "" {
		t.Error("deviceBrief 不存在设备应空")
	}
	// joinStr / joinUsed
	if joinStr([]string{"a", "b"}) != "a、b" {
		t.Errorf("joinStr=%q", joinStr([]string{"a", "b"}))
	}
	if joinUsed(nil) != "无" {
		t.Errorf("joinUsed 空应为'无'")
	}
	ju := []struct {
		MaterialName string
		Qty          int
	}{{"灯珠", 3}}
	if joinUsed(ju) != "灯珠 x3" {
		t.Errorf("joinUsed=%q", joinUsed(ju))
	}
	// extractAdv / extractAdvList
	if extractAdv("故障摘要：xxx", "故障摘要") != "xxx" {
		t.Error("extractAdv 解析失败")
	}
	// ListAdvices
	model.DB.Create(&model.AIAdvice{BizType: "fault", BizID: 1, Content: "c"})
	lst := ListAdvices("fault", 1, 5)
	if len(lst) < 1 {
		t.Error("ListAdvices 应返回")
	}
	lst2 := ListAdvices("", 0, 0)
	if len(lst2) < 1 {
		t.Error("ListAdvices 全量应返回")
	}
	lst3 := ListAdvices("fault", 0, 1000)
	if len(lst3) > 50 {
		t.Errorf("ListAdvices 限流: %d", len(lst3))
	}
}
