package service

import (
	"strings"
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

// 纯函数：时间解析（多种布局兼容）
func TestParsePatrolTime(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"2026-08-18 10:30:00", true},        // MySQL 无时区
		{"2026-08-18 10:30:00.123456", true}, // 带微秒
		{"2026-08-18T10:30:00Z", true},       // RFC3339
		{"2026-08-18T10:30:00.123Z", true},   // RFC3339Nano
		{"2026-08-18T10:30:00+08:00", true},  // 带时区偏移
		{"not-a-time", false},
		{"", false},
	}
	for _, c := range cases {
		_, err := parsePatrolTime(c.in)
		if (err == nil) != c.want {
			t.Errorf("parsePatrolTime(%q) err=%v, want success=%v", c.in, err, c.want)
		}
	}
}

// 纯函数：字符串转 uint
func TestParseUint(t *testing.T) {
	if v, err := parseUint("42"); err != nil || v != 42 {
		t.Errorf("parseUint(42) = %d,%v", v, err)
	}
	if _, err := parseUint("abc"); err == nil {
		t.Errorf("parseUint(abc) 应报错")
	}
	if v, err := parseUint("0"); err != nil || v != 0 {
		t.Errorf("parseUint(0) = %d,%v", v, err)
	}
}

// 纯函数：排行条目组装（异常率/最后巡检时间）
func TestBuildRankingItem(t *testing.T) {
	// 正常：异常率 2/5=0.4，时间可解析
	item := buildRankingItem("张三", 5, 2, "2026-08-18 10:30:00")
	if item.Key != "张三" || item.PatrolCount != 5 || item.AbnormalCnt != 2 {
		t.Fatalf("buildRankingItem 基础字段错误: %+v", item)
	}
	if item.AbnormalRate != 0.4 {
		t.Errorf("异常率应为 0.4, got %f", item.AbnormalRate)
	}
	if item.LastPatrolAt == nil {
		t.Errorf("最后巡检时间应为非空")
	}
	// 0 次巡检：异常率应为 0
	item0 := buildRankingItem("李四", 0, 0, "")
	if item0.AbnormalRate != 0 {
		t.Errorf("0 次巡检异常率应为 0, got %f", item0.AbnormalRate)
	}
	if item0.LastPatrolAt != nil {
		t.Errorf("空时间不应解析")
	}
	// 特殊时间字符串（零值）应忽略
	itemZ := buildRankingItem("王五", 3, 1, "0000-00-00 00:00:00")
	if itemZ.LastPatrolAt != nil {
		t.Errorf("零值时间不应解析")
	}
	if itemZ.AbnormalRate != 1.0/3.0 {
		t.Errorf("异常率应为 1/3, got %f", itemZ.AbnormalRate)
	}
}

// NewPatrolTaskService + 基本 CRUD（内存 SQLite）
func TestPatrolTaskCRUD(t *testing.T) {
	model.InitTestDB()
	s := NewPatrolTaskService()
	if s == nil {
		t.Fatal("NewPatrolTaskService 返回 nil")
	}

	// CreateTask 校验
	if _, err := s.CreateTask(&model.PatrolTask{}); err == nil {
		t.Error("空任务应报错")
	}
	if _, err := s.CreateTask(&model.PatrolTask{Name: "x", Mode: "badmode"}); err == nil {
		t.Error("非法模式应报错")
	}

	// 正常创建
	task := &model.PatrolTask{Name: "区域巡检", Mode: model.PatrolModeArea, CreatedBy: 1}
	created, err := s.CreateTask(task)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if created.Status != model.PatrolStatusPlanned {
		t.Errorf("默认状态应为 planned, got %s", created.Status)
	}

	// ListTasks
	tasks, total, err := s.ListTasks(1, 10)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if total != 1 || len(tasks) != 1 {
		t.Errorf("ListTasks 应返回 1 条, got total=%d len=%d", total, len(tasks))
	}

	// GetTask
	got, err := s.GetTask(created.ID, nil)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Name != "区域巡检" {
		t.Errorf("GetTask 名称不符: %s", got.Name)
	}

	// UpdateTask
	if err := s.UpdateTask(created.ID, map[string]interface{}{"status": model.PatrolStatusDone, "hack": "x"}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	got2, _ := s.GetTask(created.ID, nil)
	if got2.Status != model.PatrolStatusDone {
		t.Errorf("UpdateTask 状态未生效: %s", got2.Status)
	}
	// 无合法字段
	if err := s.UpdateTask(created.ID, map[string]interface{}{"evil": 1}); err == nil {
		t.Error("无可更新字段应报错")
	}

	// Ranking 空数据
	items, err := s.Ranking("patrol_by", 10)
	if err != nil {
		t.Fatalf("Ranking: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("空表 Ranking 应为空, got %d", len(items))
	}

	// DeleteTask
	if err := s.DeleteTask(created.ID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if _, err := s.GetTask(created.ID, nil); err == nil {
		t.Error("删除后 GetTask 应报错")
	}
}

// RunTask 执行随机抽检：离线设备标记异常
func TestPatrolTaskRunRandom(t *testing.T) {
	model.InitTestDB()
	s := NewPatrolTaskService()

	// 造 3 台设备：1 在线、2 离线
	for i := 1; i <= 3; i++ {
		d := model.Device{HwID: strings.Repeat("0", 7) + string(rune('0'+i)), OnlineStatus: i == 1}
		model.DB.Create(&d)
	}

	task := &model.PatrolTask{Name: "随机抽检", Mode: model.PatrolModeRandom, TargetCount: 3}
	created, _ := s.CreateTask(task)

	createdCnt, abnormalCnt, err := s.RunTask(created.ID, "测试员")
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	if createdCnt != 3 {
		t.Errorf("应巡检 3 台, got %d", createdCnt)
	}
	if abnormalCnt != 2 {
		t.Errorf("应异常 2 台(离线), got %d", abnormalCnt)
	}

	// 任务状态流转 done
	got, _ := s.GetTask(created.ID, nil)
	if got.Status != model.PatrolStatusDone {
		t.Errorf("任务状态应为 done, got %s", got.Status)
	}

	// Ranking 按巡检人
	items, err := s.Ranking("patrol_by", 10)
	if err != nil {
		t.Fatalf("Ranking: %v", err)
	}
	if len(items) != 1 || items[0].Key != "测试员" || items[0].PatrolCount != 3 {
		t.Errorf("Ranking 结果不符: %+v", items)
	}

	// ListRecords 过滤
	recs, rtotal, err := s.ListRecords(1, 10, map[string]string{"patrol_by": "测试员", "check_result": model.PatrolResultAbnormal})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if rtotal != 2 {
		t.Errorf("异常记录应 2 条, got %d", rtotal)
	}
	if len(recs) != 2 {
		t.Errorf("异常记录列表应 2 条, got %d", len(recs))
	}

	// 按设备排行
	ditems, err := s.Ranking("device", 10)
	if err != nil {
		t.Fatalf("Ranking(device): %v", err)
	}
	if len(ditems) != 3 {
		t.Errorf("设备排行应 3 条, got %d", len(ditems))
	}
}

// 时间过滤 + 非法 task_id 容错
func TestPatrolListRecordsFilters(t *testing.T) {
	model.InitTestDB()
	s := NewPatrolTaskService()

	task := &model.PatrolTask{Name: "t", Mode: model.PatrolModeRandom, TargetCount: 1}
	created, _ := s.CreateTask(task)

	d := model.Device{HwID: "AAA001", OnlineStatus: true}
	model.DB.Create(&d)
	s.RunTask(created.ID, "甲")

	now := time.Now()
	_, _, err := s.ListRecords(1, 10, map[string]string{
		"task_id":      "notnum",
		"device_hw_id": d.HwID,
		"start_time":   now.Add(-time.Hour).Format("2006-01-02"),
		"end_time":     now.Add(time.Hour).Format("2006-01-02"),
		"patrol_type":  model.PatrolModeRandom,
	})
	if err != nil {
		t.Fatalf("ListRecords 过滤查询: %v", err)
	}
	// 非法 task_id 被忽略而不是报错
	_, total, err := s.ListRecords(1, 10, map[string]string{"task_id": "notnum"})
	if err != nil || total < 1 {
		t.Errorf("非法 task_id 应被忽略, err=%v total=%d", err, total)
	}
}
