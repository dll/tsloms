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
	saved := model.DB
	model.DB = nil // 强制无 DB 状态（该断言依赖全局；其它 ai 测试可能已初始化，故显式置空以确保确定性）
	defer func() { model.DB = saved }()
	deny, ans := nlRequirePerm(1, "workorder:create")
	if !deny {
		t.Error("无数据库时应拒绝写命令")
	}
	if ans.Reply == "" {
		t.Error("应返回明确提示")
	}
}

// TestNlRequirePermWithDB 有数据库时：无对应业务权限的用户，NL 写命令被拒绝（P0-01）
func TestNlRequirePermWithDB(t *testing.T) {
	_ = model.InitTestDB() // 赋值全局 model.DB + AutoMigrate
	model.DB.Where("1=1").Delete(&model.User{})
	model.DB.Where("1=1").Delete(&model.UserPermission{})

	// viewer 用户，无任何覆写 → 继承 viewer 默认（无 fault:update / workorder:create）
	vw := model.User{Username: "vw_no_perm", PasswordHash: model.HashPassword("x"), Role: model.RoleViewer, Status: model.UserStatusEnabled}
	if err := model.DB.Create(&vw).Error; err != nil {
		t.Fatalf("创建 viewer 失败: %v", err)
	}

	// viewer 尝试建工单/建故障 → 均应被拒
	deny, ans := nlRequirePerm(vw.ID, "workorder:create")
	if !deny {
		t.Error("viewer 建工单应被拒绝")
	}
	if ans.Reply == "" {
		t.Error("拒绝时应给明确提示")
	}
	deny2, _ := nlRequirePerm(vw.ID, "fault:update")
	if !deny2 {
		t.Error("viewer 建故障应被拒绝")
	}

	// 管理员（内置 admin 恒有全部权限）→ 放行
	adm := model.User{Username: "adm_perm", PasswordHash: model.HashPassword("x"), Role: model.RoleAdmin, Status: model.UserStatusEnabled}
	if err := model.DB.Create(&adm).Error; err != nil {
		t.Fatalf("创建 admin 失败: %v", err)
	}
	deny3, _ := nlRequirePerm(adm.ID, "workorder:create")
	if deny3 {
		t.Error("admin 建工单不应被拒绝")
	}
}

// TestBuildAnomalyStreamEmpty 空库时异常流返回空事件与“无异常”摘要（用户级无干扰）
func TestBuildAnomalyStreamEmpty(t *testing.T) {
	_ = model.InitTestDB()
	model.DB.Where("1=1").Delete(&model.PacketLog{})
	model.DB.Where("1=1").Delete(&model.FaultRecord{})
	model.DB.Where("1=1").Delete(&model.WorkOrder{})
	model.DB.Where("1=1").Delete(&model.Device{})

	res, err := BuildAnomalyStream(24, 50)
	if err != nil {
		t.Fatalf("BuildAnomalyStream 出错: %v", err)
	}
	if res == nil {
		t.Fatal("结果不应为 nil")
	}
	if len(res.Events) != 0 {
		t.Errorf("空库应无事件, 得到 %d", len(res.Events))
	}
	if res.Summary == "" {
		t.Error("应有结论摘要")
	}
}
