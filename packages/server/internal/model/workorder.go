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

// 工单 SLA 超时阈值（单位：小时）
// - pending 超过该时长未派单 → 超时（自动升级为 processing）
// - processing 超过该时长未完成 → 超时（需管理员介入）
const (
	WorkOrderPendingSLASeconds    = 24 * 3600 // 待处理 SLA 24 小时
	WorkOrderProcessingSLASeconds = 48 * 3600 // 处理中 SLA 48 小时
)

// WorkOrderOverdueHours 计算工单是否超时及超时小时数（>0 表示超时）
// pending/processing 状态才参与超时判定，已完成/已驳回不计
func WorkOrderOverdueHours(wo *WorkOrder) float64 {
	if wo == nil {
		return 0
	}
	var sla float64
	switch wo.Status {
	case WorkOrderStatusPending:
		sla = WorkOrderPendingSLASeconds
	case WorkOrderStatusProcessing:
		sla = WorkOrderProcessingSLASeconds
	default:
		return 0
	}
	elapsed := time.Since(wo.CreatedAt).Seconds()
	overdue := elapsed - sla
	if overdue <= 0 {
		return 0
	}
	// 保留 1 位小数
	return float64(int(overdue/60)) / 60.0
}

// NextOrderNo 生成工单编号：WO{yyyyMMdd}{4位自增序号}
// 基于当日已有工单数 + 1，保证同日内序号连续且唯一
func NextOrderNo(db *gorm.DB) string {
	today := time.Now().Format("20060102")
	prefix := "WO" + today
	var count int64
	db.Model(&WorkOrder{}).Where("order_no LIKE ?", prefix+"%").Count(&count)
	return fmt.Sprintf("%s%04d", prefix, count+1)
}
