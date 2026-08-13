package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// WorkOrder 工单表
// 故障触发后自动生成维修工单，状态流转：pending → processing → completed/rejected
type WorkOrder struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	OrderNo    string     `json:"order_no" gorm:"uniqueIndex;size:32;comment:工单编号(WO{yyyyMMdd}{seq})"`
	FaultID    uint       `json:"fault_id" gorm:"index;comment:关联故障记录ID"`
	DeviceHwID uint32     `json:"device_hw_id" gorm:"index;comment:设备硬件ID"`
	Status     string     `json:"status" gorm:"size:16;default:pending;comment:状态(pending/processing/completed/rejected)"`
	AssigneeID *uint      `json:"assignee_id" gorm:"comment:处理人ID"`
	Result     string     `json:"result" gorm:"type:text;comment:维修结果说明"`
	CreatedAt  time.Time  `json:"created_at"`
	ClosedAt   *time.Time `json:"closed_at" gorm:"comment:闭环时间"`
}

// TableName 指定表名
func (WorkOrder) TableName() string {
	return "work_orders"
}

// 工单状态常量
const (
	WorkOrderStatusPending    = "pending"
	WorkOrderStatusProcessing = "processing"
	WorkOrderStatusCompleted  = "completed"
	WorkOrderStatusRejected   = "rejected"
)

// NextOrderNo 生成工单编号：WO{yyyyMMdd}{4位自增序号}
// 基于当日已有工单数 + 1，保证同日内序号连续且唯一
func NextOrderNo(db *gorm.DB) string {
	today := time.Now().Format("20060102")
	prefix := "WO" + today
	var count int64
	db.Model(&WorkOrder{}).Where("order_no LIKE ?", prefix+"%").Count(&count)
	return fmt.Sprintf("%s%04d", prefix, count+1)
}
