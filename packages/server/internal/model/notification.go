package model

import (
	"time"

	"gorm.io/gorm"
)

// Notification 站内通知：AI 主动巡检推送（运维日报提醒 / 异常预警）等
// type: report(报告已生成) / alert(异常预警) / system(系统)
// scope: all(全体相关用户) / user(指定用户)
// 广播通知（user_id=0）的已读状态记录在 NotificationRead 表，实现用户级隔离（P1-02）
type Notification struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index;comment:接收用户(0=全体广播)"`
	Type      string    `json:"type" gorm:"size:16;default:system;comment:通知类型(report/alert/system)"`
	Title     string    `json:"title" gorm:"size:128;comment:标题"`
	Content   string    `json:"content" gorm:"size:1024;comment:内容"`
	Link      string    `json:"link" gorm:"size:256;comment:跳转链接(前端路由)"`
	BizType   string    `json:"biz_type" gorm:"size:32;comment:关联业务类型"`
	BizID     uint      `json:"biz_id" gorm:"comment:关联业务ID"`
	IsRead    bool      `json:"is_read" gorm:"default:false;index;comment:是否已读(仅针对 user_id>0 的单人通知)"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName
func (Notification) TableName() string { return "notifications" }

// NotificationRead 广播通知(user_id=0)的按用户已读状态（用户级隔离）
type NotificationRead struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	NotificationID uint      `json:"notification_id" gorm:"index:idx_nr_notif_user,unique,comment:通知ID"`
	UserID         uint      `json:"user_id" gorm:"index:idx_nr_notif_user,unique;comment:用户ID"`
	ReadAt         time.Time `json:"read_at"`
}

// TableName
func (NotificationRead) TableName() string { return "notification_reads" }

// CreateNotification 创建站内通知
// userID=0 表示面向全体用户（各用户已读状态存 NotificationRead 隔离）
func CreateNotification(userID uint, ntype, title, content, link, bizType string, bizID uint) {
	rec := &Notification{
		UserID: userID, Type: ntype, Title: title, Content: content,
		Link: link, BizType: bizType, BizID: bizID, IsRead: false,
	}
	DB.Create(rec)
}

// UnreadNotificationCount 用户未读通知数（面向本人的 is_read + 面向全体的未读记录）
// 广播通知按 user_id=0 且当前用户无 NotificationRead 记录计为未读，实现用户级隔离
func UnreadNotificationCount(userID uint) int64 {
	var n int64
	// 1) 面向本人的未读
	DB.Model(&Notification{}).
		Where("is_read = ? AND user_id = ?", false, userID).
		Count(&n)
	// 2) 面向全体(广播)、当前用户未读（在 notification_reads 中无记录）
	var b int64
	DB.Model(&Notification{}).
		Where("user_id = 0 AND id NOT IN (SELECT notification_id FROM notification_reads WHERE user_id = ?)", userID).
		Count(&b)
	return n + b
}

// ListUserNotifications 用户通知列表（本人的 + 面向全体的），广播通知按当前用户回填 is_read
func ListUserNotifications(userID uint, limit int) []Notification {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var list []Notification
	DB.Where("user_id = ? OR user_id = 0", userID).
		Order("created_at DESC").Limit(limit).Find(&list)

	// 广播通知按当前用户已读记录回填 is_read（不修改库中共享行）
	for i := range list {
		if list[i].UserID == 0 {
			var cnt int64
			DB.Model(&NotificationRead{}).
				Where("notification_id = ? AND user_id = ?", list[i].ID, userID).
				Count(&cnt)
			list[i].IsRead = cnt > 0
		}
	}
	return list
}

// MarkNotificationRead 标记单条通知已读（用户级；广播通知写入 notification_reads 隔离）
// 返回 (标记数, 错误)；不存在的通知返回 (0, nil)
func MarkNotificationRead(userID, notifID uint) (int64, error) {
	var notif Notification
	if err := DB.First(&notif, notifID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	// 非本人且非广播：无权标记（返回成功但不改动）
	if notif.UserID != 0 && notif.UserID != userID {
		return 0, nil
	}
	if notif.UserID == 0 {
		// 广播通知：插入/更新用户已读记录（幂等）
		err := DB.Where("notification_id = ? AND user_id = ?", notifID, userID).
			Assign(NotificationRead{NotificationID: notifID, UserID: userID, ReadAt: time.Now()}).
			FirstOrCreate(&NotificationRead{}).Error
		if err != nil {
			return 0, err
		}
		return 1, nil
	}
	// 单人通知：直接更新 is_read
	tx := DB.Model(&Notification{}).Where("id = ?", notifID).Update("is_read", true)
	return tx.RowsAffected, tx.Error
}

// MarkAllNotificationsRead 全部标记已读（用户级隔离）
func MarkAllNotificationsRead(userID uint) error {
	// 1) 单人通知置已读
	tx := DB.Model(&Notification{}).Where("is_read = ? AND user_id = ?", false, userID).Update("is_read", true)
	if tx.Error != nil {
		return tx.Error
	}
	// 2) 广播通知批量写入当前用户的已读记录（NOT EXISTS 防重复）
	return DB.Exec(`INSERT INTO notification_reads (notification_id, user_id, read_at)
		SELECT id, ?, ? FROM notifications WHERE user_id = 0
		AND id NOT IN (SELECT notification_id FROM notification_reads WHERE user_id = ?)`,
		userID, time.Now(), userID).Error
}
