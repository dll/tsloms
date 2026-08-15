package service

import (
	"testing"

	"github.com/tsloms/server/internal/model"
)

// 确保巡检通知能正确创建并查询（针对运维/管理员定向 + 未读计数）
func TestPatrolNotifyAndRead(t *testing.T) {
	if model.DB == nil {
		t.Skip("无数据库")
	}
	// 创建一个测试用户作为接收方（role=operator）
	u := model.User{Username: "patrol_test_op", Role: "operator", Status: model.UserStatusEnabled, PasswordHash: "x"}
	if err := model.DB.Create(&u).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	defer model.DB.Unscoped().Delete(&u)

	// 创建通知
	model.CreateNotification(u.ID, "report", "AI 巡检日报", "测试日报内容", "/ai/workbench", "report", 0)
	model.CreateNotification(u.ID, "alert", "工单超时", "有工单超时", "/workorder", "workorder", 0)

	// 未读计数
	if n := model.UnreadNotificationCount(u.ID); n != 2 {
		t.Fatalf("未读计数应为2, got %d", n)
	}

	// 标记一条已读
	var list []model.Notification
	model.DB.Where("user_id = ?", u.ID).Find(&list)
	if len(list) != 2 {
		t.Fatalf("应创建2条通知, got %d", len(list))
	}
	model.DB.Model(&model.Notification{}).Where("id = ?", list[0].ID).Update("is_read", true)
	if n := model.UnreadNotificationCount(u.ID); n != 1 {
		t.Fatalf("标记已读后未读应为1, got %d", n)
	}

	// 列表查询
	notifs := model.ListUserNotifications(u.ID, 10)
	if len(notifs) != 2 {
		t.Fatalf("列表应含2条, got %d", len(notifs))
	}
	// 清理
	model.DB.Where("user_id = ?", u.ID).Delete(&model.Notification{})
}

// 定向推送：无运维/管理员用户时退化为面向全体
// （独立针对 CreateNotification(user_id=0) 覆盖全体可见性，不清理真实用户）
func TestPatrolNotifyBroadcast(t *testing.T) {
	if model.DB == nil {
		t.Skip("无数据库")
	}
	// 面向全体(user_id=0)通知，任意用户均可查询到
	model.CreateNotification(0, "alert", "缺货", "缺货提醒", "/inventory/material", "inventory", 0)
	if n := model.UnreadNotificationCount(99999); n < 1 {
		t.Fatalf("面向全体通知应对任意用户可见, got %d", n)
	}
	list := model.ListUserNotifications(99999, 10)
	if len(list) < 1 {
		t.Fatalf("面向全体通知应在任意用户列表中, got %d", len(list))
	}
	// 清理所有 user_id=0 通知
	model.DB.Where("user_id = ?", 0).Delete(&model.Notification{})
}
