package model

import (
	"time"
)

// Notification 站内通知：AI 主动巡检推送（运维日报提醒 / 异常预警）等
// type: report(报告已生成) / alert(异常预警) / system(系统)
// scope: all(全体相关用户) / user(指定用户)
type Notification struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index;comment:接收用户(0=全体)"`
	Type      string    `json:"type" gorm:"size:16;default:system;comment:通知类型(report/alert/system)"`
	Title     string    `json:"title" gorm:"size:128;comment:标题"`
	Content   string    `json:"content" gorm:"size:1024;comment:内容"`
	Link      string    `json:"link" gorm:"size:256;comment:跳转链接(前端路由)"`
	BizType   string    `json:"biz_type" gorm:"size:32;comment:关联业务类型"`
	BizID     uint      `json:"biz_id" gorm:"comment:关联业务ID"`
	IsRead    bool      `json:"is_read" gorm:"default:false;index;comment:是否已读"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName
func (Notification) TableName() string { return "notifications" }

// CreateNotification 创建站内通知
// userID=0 表示面向全体用户（各用户通过 GetNotifications 时按 scope 展开）
func CreateNotification(userID uint, ntype, title, content, link, bizType string, bizID uint) {
	rec := &Notification{
		UserID: userID, Type: ntype, Title: title, Content: content,
		Link: link, BizType: bizType, BizID: bizID, IsRead: false,
	}
	DB.Create(rec)
}

// UnreadNotificationCount 用户未读通知数（面向本人的 + 面向全体的）
func UnreadNotificationCount(userID uint) int64 {
	var n int64
	DB.Model(&Notification{}).
		Where("is_read = ? AND (user_id = ? OR user_id = 0)", false, userID).
		Count(&n)
	return n
}

// ListUserNotifications 用户通知列表（本人的 + 面向全体的）
func ListUserNotifications(userID uint, limit int) []Notification {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var list []Notification
	DB.Where("user_id = ? OR user_id = 0", userID).
		Order("created_at DESC").Limit(limit).Find(&list)
	return list
}
