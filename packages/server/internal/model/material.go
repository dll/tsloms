package model

import "time"

// DeviceMaterial 设备维修耗材台账
// 记录路灯设备使用/备用的耗材（灯泡型号、电源、控制器等），用于维修派单参考
type DeviceMaterial struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	DeviceHwID    uint32    `json:"device_hw_id" gorm:"index;comment:设备硬件ID"`
	Name          string    `json:"name" gorm:"size:64;comment:耗材名称"`
	PartNo        string    `json:"part_no" gorm:"size:64;comment:型号/料号"`
	Spec          string    `json:"spec" gorm:"size:128;comment:规格参数"`
	Quantity      int       `json:"quantity" gorm:"default:0;comment:当前库存"`
	Unit          string    `json:"unit" gorm:"size:16;comment:单位(个/支/套)"`
	Threshold     int       `json:"threshold" gorm:"default:0;comment:库存预警阈值"`
	LastChangedAt *time.Time `json:"last_changed_at" gorm:"comment:最近变动时间"`
	Note          string    `json:"note" gorm:"type:text;comment:备注"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName 指定表名
func (DeviceMaterial) TableName() string {
	return "device_materials"
}
