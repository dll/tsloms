package service

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

// TestWorkOrderEscalator_AutoUpgradePending 验证超时的 pending 工单被自动升级为 processing
func TestWorkOrderEscalator_AutoUpgradePending(t *testing.T) {
	model.InitTestDB()
	// 清空工单表，保证可重复运行
	model.DB.Exec("DELETE FROM work_orders")

	// 一张超时 pending（创建于 3 天前，超过 24h SLA）
	overdue := model.WorkOrder{
		OrderNo:   "WO-TEST-OVERDUE",
		FaultID:   1,
		Status:    model.WorkOrderStatusPending,
		CreatedAt: time.Now().Add(-72 * time.Hour),
	}
	if err := model.DB.Create(&overdue).Error; err != nil {
		t.Fatalf("创建超时工单失败: %v", err)
	}
	// 一张未超时 pending（刚创建，不应被升级）
	fresh := model.WorkOrder{
		OrderNo:   "WO-TEST-FRESH",
		FaultID:   2,
		Status:    model.WorkOrderStatusPending,
		CreatedAt: time.Now(),
	}
	if err := model.DB.Create(&fresh).Error; err != nil {
		t.Fatalf("创建新工单失败: %v", err)
	}
	// 一张已完成工单（不应受影响）
	done := model.WorkOrder{
		OrderNo: "WO-TEST-DONE",
		FaultID: 3,
		Status:  model.WorkOrderStatusCompleted,
		CreatedAt: func() time.Time {
			now := time.Now()
			return now.Add(-100 * time.Hour)
		}(),
	}
	if err := model.DB.Create(&done).Error; err != nil {
		t.Fatalf("创建完成工单失败: %v", err)
	}

	e := NewWorkOrderEscalator()
	e.runOnce()

	t.Logf("IDs: overdue=%d fresh=%d done=%d", overdue.ID, fresh.ID, done.ID)
	// 逐行校验
	for _, tc := range []struct {
		id    uint
		want  string
		label string
	}{{overdue.ID, model.WorkOrderStatusProcessing, "超时pending应升级"}, {fresh.ID, model.WorkOrderStatusPending, "未超时工单不应升级"}, {done.ID, model.WorkOrderStatusCompleted, "已完成工单不应被改动"}} {
		var row model.WorkOrder
		model.DB.First(&row, tc.id)
		if row.Status != tc.want {
			t.Errorf("%s: id=%d got=%s want=%s", tc.label, tc.id, row.Status, tc.want)
		}
	}
	// 校验升级的工单写入了提示信息
	var upgraded model.WorkOrder
	model.DB.First(&upgraded, overdue.ID)
	if upgraded.Result == "" {
		t.Error("升级后应写入提示信息(Result)")
	}
}

// TestWorkOrderEscalator_NoDB 无 DB 时应安全返回（不 panic）
func TestWorkOrderEscalator_NoDB(t *testing.T) {
	oldDB := model.DB
	model.DB = nil
	defer func() { model.DB = oldDB }()
	e := NewWorkOrderEscalator()
	e.runOnce() // 不应 panic
}
