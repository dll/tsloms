package ai

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

// nlSeed 填充 NL 工具执行所需数据
func nlSeed(t *testing.T) {
	t.Helper()
	model.InitTestDB()
	op := model.User{Username: "op_nl", PasswordHash: "x", Role: model.RoleOperator}
	model.DB.Create(&op)
	opID = op.ID
	// viewer（用于权限拒绝测试）
	vw := model.User{Username: "vw_nl", PasswordHash: "x", Role: model.RoleViewer}
	model.DB.Create(&vw)
	vwNlID = vw.ID
	model.DB.Create(&model.Device{HwID: 5001, Intersection: "人民路口", OnlineStatus: true})
	model.DB.Create(&model.Device{HwID: 5002, Intersection: "建设路口", OnlineStatus: false})
	now := time.Now()
	model.DB.Create(&model.FaultRecord{DeviceHwID: 5001, FaultType: "lamp_off", Status: model.FaultStatusOccurred, FirstSeen: now, LastSeen: now})
	model.DB.Create(&model.WorkOrder{OrderNo: "WOnl1", DeviceHwID: 5001, Status: model.WorkOrderStatusPending, CreatedAt: now.Add(-30 * time.Hour)})
	model.DB.Create(&model.RepairExpense{ExpenseNo: "FEnl", Type: "material", Amount: 500, DeviceHwID: 5001, CreatedAt: now})
}

func TestInterpretNL_QueryTools(t *testing.T) {
	nlSeed(t)
	// 故障排行
	ans := InterpretNL(1, "最近7天哪些路口故障最多")
	if ans.Tool != "fault_rank" || ans.Data == nil {
		t.Errorf("fault_rank: tool=%s data=%v", ans.Tool, ans.Data)
	}
	// 设备状态
	ans = InterpretNL(1, "查询设备5001状态")
	if ans.Tool != "device_status" || len(ans.Data["list"].([]map[string]any)) == 0 {
		t.Errorf("device_status: tool=%s", ans.Tool)
	}
	// 工单统计
	ans = InterpretNL(1, "工单统计")
	if ans.Tool != "workorder_stats" {
		t.Errorf("workorder_stats: tool=%s", ans.Tool)
	}
	// 费用
	ans = InterpretNL(1, "最近30天费用")
	if ans.Tool != "expense_summary" {
		t.Errorf("expense_summary: tool=%s", ans.Tool)
	}
	// 健康评分
	ans = InterpretNL(1, "运维健康评分是多少")
	if ans.Tool != "ops_health" || ans.Data == nil {
		t.Errorf("ops_health: tool=%s", ans.Tool)
	}
	// 异常流
	ans = InterpretNL(1, "最近有哪些异常告警")
	if ans.Tool != "anomaly_stream" {
		t.Errorf("anomaly_stream: tool=%s", ans.Tool)
	}
}

func TestInterpretNL_CreateFault(t *testing.T) {
	nlSeed(t)
	ans := InterpretNL(1, "报修：人民路口红灯不亮")
	if !ans.DidWrite || ans.Tool != "create_fault" {
		t.Fatalf("应创建故障单: tool=%s did_write=%v", ans.Tool, ans.DidWrite)
	}
	if ans.CreatedID == 0 {
		t.Error("应返回故障单ID")
	}
	// 设备不存在
	ans2 := InterpretNL(1, "报修：不存在路口黄灯不亮")
	if ans2.DidWrite {
		t.Errorf("设备不存在不应写, reply=%s", ans2.Reply)
	}
	// 权限拒绝（viewer 建故障）
	_ = opID
	ans3 := InterpretNL(vwNlID, "报修：人民路口红灯不亮")
	if ans3.DidWrite || ans3.Reply == "" {
		t.Errorf("viewer 建故障应被拒: %+v", ans3)
	}
}

func TestInterpretNL_CreateWorkOrder(t *testing.T) {
	nlSeed(t)
	ans := InterpretNL(1, "给设备5001建工单")
	if !ans.DidWrite || ans.Tool != "create_workorder" {
		t.Fatalf("应创建工单: tool=%s did_write=%v reply=%s", ans.Tool, ans.DidWrite, ans.Reply)
	}
	if ans.CreatedID == 0 {
		t.Error("应返回工单ID")
	}
	// 设备不存在
	ans2 := InterpretNL(1, "给设备999建工单")
	if ans2.DidWrite {
		t.Errorf("设备不存在不应写: %s", ans2.Reply)
	}
	// viewer 权限拒绝
	ans3 := InterpretNL(vwNlID, "给设备5001建工单")
	if ans3.DidWrite {
		t.Errorf("viewer 建工单应被拒: %s", ans3.Reply)
	}
}

func TestInterpretNL_KnowledgeAndFallback(t *testing.T) {
	nlSeed(t)
	// 知识库（如何新建工单）
	ans := InterpretNL(1, "怎么新建工单")
	if ans.Tool != "kb" {
		t.Errorf("操作咨询应走知识库, tool=%s", ans.Tool)
	}
	// 询问供应商
	ans2 := InterpretNL(1, "采购怎么操作")
	if ans2.Reply == "" {
		t.Error("知识库应有回答")
	}
	// 未知 → 兜底
	ans3 := InterpretNL(1, "今天天气怎么样")
	if ans3.Tool != "kb" {
		t.Errorf("未知应走兜底, tool=%s", ans3.Tool)
	}
}

func TestNlRequirePerm_WithDB(t *testing.T) {
	nlSeed(t)
	// viewer 无 fault:update → 拒绝
	deny, ans := nlRequirePerm(vwNlID, "fault:update")
	if !deny || ans.Reply == "" {
		t.Errorf("viewer 应被拒 fault:update, deny=%v", deny)
	}
	// operator 有（通过 EffectivePermissions）
	op := model.User{}
	model.DB.Where("username = ?", "op_nl").First(&op)
	deny2, _ := nlRequirePerm(op.ID, "fault:update")
	if deny2 {
		t.Errorf("operator 应有 fault:update 权限")
	}
	// 不存在用户 → 拒绝
	deny3, _ := nlRequirePerm(99999, "fault:update")
	if !deny3 {
		t.Error("不存在用户应拒绝")
	}
}

// vwNlID / opID 测试辅助：从 DB 取测试用户 ID（惰性，测试内初始化 DB 后调用）
var vwNlID uint
var opID uint
