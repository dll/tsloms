package model

import (
	"testing"
)

// TestNotificationBroadcastReadIsolation 验证广播通知(user_id=0)的已读状态按用户隔离（P1-02）
// 至少两个用户：用户A标记已读后，用户B的未读数与已读状态不受影响
func TestNotificationBroadcastReadIsolation(t *testing.T) {
	db := InitTestDB()
	_ = db

	// 清空通知与已读表，保证从干净状态开始
	DB.Exec("DELETE FROM notification_reads")
	DB.Exec("DELETE FROM notifications")

	// 创建两条广播通知（面向全体）
	CreateNotification(0, "alert", "广播A", "全体异常预警A", "", "alert", 0)
	CreateNotification(0, "report", "广播B", "全体日报B", "", "report", 0)
	var b1, b2 Notification
	if err := DB.Where("title = ?", "广播A").First(&b1).Error; err != nil {
		t.Fatalf("查询广播A失败: %v", err)
	}
	if err := DB.Where("title = ?", "广播B").First(&b2).Error; err != nil {
		t.Fatalf("查询广播B失败: %v", err)
	}

	const userA, userB = uint(101), uint(102)

	// 初始：两用户各 2 条未读
	if n := UnreadNotificationCount(userA); n != 2 {
		t.Fatalf("初始 userA 未读数 = %d, want 2", n)
	}
	if n := UnreadNotificationCount(userB); n != 2 {
		t.Fatalf("初始 userB 未读数 = %d, want 2", n)
	}

	// 用户A 标记“广播A”已读
	if n, err := MarkNotificationRead(userA, b1.ID); err != nil || n != 1 {
		t.Fatalf("userA mark read: n=%d err=%v, want 1, nil", n, err)
	}

	// userA 未读降为 1；userB 仍为 2（隔离生效）
	if n := UnreadNotificationCount(userA); n != 1 {
		t.Fatalf("userA 已读后未读数 = %d, want 1", n)
	}
	if n := UnreadNotificationCount(userB); n != 2 {
		t.Fatalf("userB 未读数被误影响 = %d, want 2", n)
	}

	// 列表回填：广播A 对 A 已读、对 B 未读
	listA := ListUserNotifications(userA, 10)
	foundA1 := false
	for _, n := range listA {
		if n.ID == b1.ID {
			foundA1 = true
			if !n.IsRead {
				t.Fatalf("列表A 广播A 应为已读")
			}
		}
	}
	if !foundA1 {
		t.Fatalf("userA 列表缺少广播A")
	}
	listB := ListUserNotifications(userB, 10)
	for _, n := range listB {
		if n.ID == b1.ID && n.IsRead {
			t.Fatalf("列表B 广播A 不应为已读（用户级隔离）")
		}
	}

	// 用户B 全部标记已读：B 未读归 0，A 未读仍为 1（互不影响）
	if err := MarkAllNotificationsRead(userB); err != nil {
		t.Fatalf("userB markAll: %v", err)
	}
	if n := UnreadNotificationCount(userB); n != 0 {
		t.Fatalf("userB 全部已读后未读数 = %d, want 0", n)
	}
	if n := UnreadNotificationCount(userA); n != 1 {
		t.Fatalf("userA 未读数被 userB 误影响 = %d, want 1", n)
	}
	if n := UnreadNotificationCount(userA); n != 1 {
		t.Fatalf("userA 未读数被 userB 误影响(再次) = %d, want 1", n)
	}
}

// TestNotificationTargetedRead 单人通知(user_id>0)已读仅影响本人
func TestNotificationTargetedRead(t *testing.T) {
	db := InitTestDB()
	_ = db
	DB.Exec("DELETE FROM notification_reads")
	DB.Exec("DELETE FROM notifications")

	const userA, userB = uint(201), uint(202)
	CreateNotification(userA, "system", "给A的通知", "仅A可见", "", "", 0)
	CreateNotification(0, "alert", "广播C", "全体", "", "", 0)

	if n := UnreadNotificationCount(userB); n != 1 {
		t.Fatalf("userB 未读数 = %d, want 1(仅广播C)", n)
	}

	// userB 无权改 userA 的单人通知
	var a Notification
	if err := DB.Where("title = ?", "给A的通知").First(&a).Error; err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n, err := MarkNotificationRead(userB, a.ID); err != nil || n != 0 {
		t.Fatalf("userB 标记 A 的单人通知: n=%d err=%v, want 0,nil", n, err)
	}
	// userA 初始未读 = 单人通知 + 广播C = 2，且未被 userB 改动
	if n := UnreadNotificationCount(userA); n != 2 {
		t.Fatalf("userA 未读(单人通知未被B改) = %d, want 2", n)
	}

	// userA 标记自己的单人通知已读后，仅剩广播C 未读
	if n, err := MarkNotificationRead(userA, a.ID); err != nil || n != 1 {
		t.Fatalf("userA 标记本人通知: n=%d err=%v, want 1,nil", n, err)
	}
	if n := UnreadNotificationCount(userA); n != 1 {
		t.Fatalf("userA 未读(已读单人通知) = %d, want 1(仅广播C)", n)
	}
}
