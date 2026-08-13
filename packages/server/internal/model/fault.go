package model

import "time"

// FaultRecord 故障记录表
// 记录设备上报的故障信息，同一设备同一 errCode 30分钟内去重
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
	Status      string    `json:"status" gorm:"size:16;default:active;comment:状态(active/resolved)"`
	WorkOrderID *uint     `json:"work_order_id" gorm:"comment:关联工单ID"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 指定表名
func (FaultRecord) TableName() string {
	return "fault_records"
}
