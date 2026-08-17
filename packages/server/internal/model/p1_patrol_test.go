package model

import "testing"

// ---- P1 自动巡检：表结构与常量 ----

func TestPatrolTask_Constants(t *testing.T) {
	modes := []string{PatrolModeArea, PatrolModeStreet, PatrolModeRandom, PatrolModeSelfCheck, PatrolModeAI}
	for _, m := range modes {
		if m == "" {
			t.Errorf("巡检模式常量不能为空")
		}
	}
	statuses := []string{PatrolStatusPlanned, PatrolStatusRunning, PatrolStatusDone}
	for _, s := range statuses {
		if s == "" {
			t.Errorf("任务状态常量不能为空")
		}
	}
	if PatrolResultNormal == PatrolResultAbnormal {
		t.Errorf("normal 与 abnormal 不得相同")
	}
}

func TestPatrolTask_TableNames(t *testing.T) {
	if (&PatrolTask{}).TableName() != "patrol_tasks" {
		t.Errorf("PatrolTask 表名应为 patrol_tasks")
	}
	if (&PatrolRecord{}).TableName() != "patrol_records" {
		t.Errorf("PatrolRecord 表名应为 patrol_records")
	}
}

func TestPatrolModel_AutoMigrateCreatesTables(t *testing.T) {
	db := InitTestDB()
	// 自动化迁移后两张表应存在
	if !db.Migrator().HasTable(&PatrolTask{}) {
		t.Errorf("patrol_tasks 表应被 AutoMigrate 创建")
	}
	if !db.Migrator().HasTable(&PatrolRecord{}) {
		t.Errorf("patrol_records 表应被 AutoMigrate 创建")
	}

	// 任务读写
	task := &PatrolTask{Name: "迁移测试", Mode: PatrolModeRandom, TargetCount: 1, Status: PatrolStatusPlanned}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	var got PatrolTask
	if err := db.First(&got, task.ID).Error; err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	if got.Name != "迁移测试" || got.Mode != PatrolModeRandom {
		t.Errorf("任务读写内容不符: %+v", got)
	}

	// 记录读写
	rec := &PatrolRecord{TaskID: &task.ID, DeviceHwID: 1, PatrolType: PatrolModeRandom, CheckResult: PatrolResultNormal}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("创建记录失败: %v", err)
	}
	var gotRec PatrolRecord
	if err := db.First(&gotRec, rec.ID).Error; err != nil {
		t.Fatalf("读取记录失败: %v", err)
	}
	if gotRec.DeviceHwID != 1 || gotRec.CheckResult != PatrolResultNormal {
		t.Errorf("记录读写内容不符: %+v", gotRec)
	}
}
