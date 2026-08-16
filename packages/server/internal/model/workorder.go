package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// WorkOrder 工单表
// 故障触发后自动生成维修工单，状态流转：pending → processing → completed/rejected
// FaultActiveScope 用于 MySQL 兼容的「同一故障至多一条活跃工单」约束（M1）：
//   活跃工单(pending/processing)：FaultActiveScope = fault_id；完结/驳回：FaultActiveScope = NULL。
//   配合唯一索引 uk_wo_active_scope（NULL 不参与唯一，MySQL/SQLite 均允许多个 NULL），
//   从 DB 层保证同一 fault_id 至多一条活跃工单，防止并发复核/自动派单重复建单。
//   （MySQL 不支持 SQLite/Postgres 的“部分/过滤索引”，故用可空派生列模拟；见 migrate.go）
type WorkOrder struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	OrderNo    string     `json:"order_no" gorm:"uniqueIndex;size:32;comment:工单编号(WO{yyyyMMdd}{seq})"`
	FaultID    uint       `json:"fault_id" gorm:"index;comment:关联故障记录ID"`
	DeviceHwID uint32     `json:"device_hw_id" gorm:"index;comment:设备硬件ID"`
	Status     string     `json:"status" gorm:"size:16;default:pending;comment:状态(pending/processing/completed/rejected)"`
	// FaultActiveScope 活跃工单唯一约束载体：active=pending/processing 时为 fault_id；inactive 为 NULL
	// 注：不用 uniqueIndex tag——唯一索引由 migrate.go 在建列并清理/回填后手动创建，避免 AutoMigrate 在重复数据上建唯一失败
	FaultActiveScope *uint      `json:"fault_active_scope,omitempty" gorm:"index;comment:活跃工单唯一约束(fault_id);非活跃为NULL(唯一索引见migrate.go)"`
	AssigneeID        *uint     `json:"assignee_id" gorm:"comment:处理人ID"`
	Result            string    `json:"result" gorm:"type:text;comment:维修结果说明"`
	CreatedAt         time.Time `json:"created_at"`
	ClosedAt          *time.Time `json:"closed_at" gorm:"comment:闭环时间"`
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

// EnsureActiveWorkOrder 为故障原子式创建/复用一条活跃工单（pending），并回填 fault.work_order_id。
// 并发安全（M1）：依赖 work_orders 的唯一索引 uk_wo_active_scope(fault_active_scope) 作为数据库层闸门——
// 活跃工单 FaultActiveScope=fault_id 唯一，NULL(非活跃/完结/驳回) 不参与唯一；
// 配合 fault_records.work_order_id 的条件更新（WHERE work_order_id IS NULL）做应用层抢锁，
// 保证同一故障无论从多少入口（processFault 自动派单 / ReviewFault 复核派单）并发触发，最终只建成一条活跃工单。
// 返回最终生效的工单；建单失败且无既有活跃单可复用时返回 nil。
func EnsureActiveWorkOrder(db *gorm.DB, faultID uint, deviceHwID uint32) *WorkOrder {
	if db == nil {
		return nil
	}

	// 1) 尝试原子创建。冲突（唯一索引拦截或回填被抢先）时回退复用已有活跃单。
	wo := &WorkOrder{
		OrderNo:          NextOrderNo(db),
		FaultID:          faultID,
		DeviceHwID:       deviceHwID,
		Status:           WorkOrderStatusPending,
		FaultActiveScope: &faultID, // 活跃工单占据 fault 唯一位
	}
	if err := db.Create(wo).Error; err != nil {
		// 创建失败（唯一索引冲突或其他）：退化为查询已存在的活跃单并复用
		var existing WorkOrder
		if err2 := db.Where("fault_id = ? AND fault_active_scope = ?", faultID, faultID).
			First(&existing).Error; err2 == nil {
			// 复用已存在活跃单
			wo = &existing
		} else {
			return nil
		}
	}

	// 2) 回填 fault.work_order_id 与状态（条件更新保证并发下不覆盖他人的单）
	now := time.Now()
	db.Model(&FaultRecord{}).
		Where("id = ? AND work_order_id IS NULL", faultID).
		Updates(map[string]interface{}{
			"work_order_id": wo.ID,
			"status":        FaultStatusConfirmed,
			"confirmed_at":  &now,
		})

	return wo
}
