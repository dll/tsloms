package model

import "time"

// FaultRecord 故障记录表
// 记录设备上报的故障信息，同一设备同一 errCode 30分钟内去重
// 生命周期状态机：occurred(发生) → confirmed(确认) → dispatched(已派单) → resolved(已解决)
type FaultRecord struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	DeviceHwID  uint32    `json:"device_hw_id" gorm:"index;comment:设备硬件ID"`
	ErrCode     int8      `json:"err_code" gorm:"comment:错误码(-1至-14)"`
	FaultType   string    `json:"fault_type" gorm:"size:32;comment:故障类型分类"`
	FaultLevel  string    `json:"fault_level" gorm:"size:16;comment:故障等级(critical/normal)"`
	LedState    int8      `json:"led_state" gorm:"comment:故障时灯组状态"`
	CurrentR    uint16    `json:"current_r" gorm:"comment:红灯电流值"`
	CurrentY    uint16    `json:"current_y" gorm:"comment:黄灯电流值"`
	CurrentG    uint16    `json:"current_g" gorm:"comment:绿灯电流值"`
	FirstSeen   time.Time `json:"first_seen" gorm:"comment:首次出现时间"`
	LastSeen    time.Time `json:"last_seen" gorm:"comment:最后出现时间"`
	Status      string    `json:"status" gorm:"size:16;default:occurred;comment:状态(occurred/confirmed/dispatched/resolved)"`
	OwnerID     *uint     `json:"owner_id" gorm:"comment:负责人ID(故障确认人)"`
	RepairerID  *uint     `json:"repairer_id" gorm:"comment:维修人ID"`
	ConfirmedAt *time.Time `json:"confirmed_at" gorm:"comment:确认时间"`
	DispatchedAt *time.Time `json:"dispatched_at" gorm:"comment:派单时间"`
	ResolvedAt  *time.Time `json:"resolved_at" gorm:"comment:解决时间"`
	WorkOrderID *uint     `json:"work_order_id" gorm:"comment:关联工单ID"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// 故障生命周期状态常量
const (
	FaultStatusOccurred   = "occurred"   // 发生
	FaultStatusConfirmed  = "confirmed"  // 已确认
	FaultStatusDispatched = "dispatched" // 已派单
	FaultStatusResolved   = "resolved"   // 已解决
)

// TableName 指定表名
func (FaultRecord) TableName() string {
	return "fault_records"
}

// IsActive 判断故障是否仍处于进行中（未解决）
func (f *FaultRecord) IsActive() bool {
	return f.Status != FaultStatusResolved
}
