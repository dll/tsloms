package model

import "time"

// Feedback 问题反馈
// 地图/移动端/后台提交的设备或路口问题反馈，可关联工单
type Feedback struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	DeviceHwID    *uint32   `json:"device_hw_id" gorm:"index;comment:关联设备硬件ID(可空)"`
	Intersection  string    `json:"intersection" gorm:"size:128;comment:路口位置"`
	Title         string    `json:"title" gorm:"size:128;comment:反馈标题"`
	Content       string    `json:"content" gorm:"type:text;comment:反馈内容"`
	Reporter      string    `json:"reporter" gorm:"size:64;comment:反馈人"`
	Contact       string    `json:"contact" gorm:"size:64;comment:联系方式"`
	Status        string    `json:"status" gorm:"size:16;default:open;comment:状态(open/processing/resolved/closed)"`
	WorkOrderID   *uint     `json:"work_order_id" gorm:"comment:关联工单ID"`
	ImageURL      string    `json:"image_url" gorm:"size:512;comment:反馈图片URL"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Feedback) TableName() string {
	return "feedbacks"
}

// 反馈状态常量
const (
	FeedbackOpen        = "open"        // 待处理
	FeedbackProcessing  = "processing"  // 处理中
	FeedbackResolved    = "resolved"    // 已解决
	FeedbackClosed      = "closed"      // 已关闭
)
