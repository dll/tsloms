package model

import "time"

// Device 设备表
// 记录信号灯监控设备的台账信息，hw_id 为出厂唯一硬件 ID
type Device struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	HwID          uint32    `json:"hw_id" gorm:"uniqueIndex;comment:设备硬件ID(出厂唯一)"`
	Intersection  string    `json:"intersection" gorm:"size:128;comment:路口位置描述"`
	NetworkCode   int       `json:"network_code" gorm:"comment:网络号"`
	StationCode   int       `json:"station_code" gorm:"comment:站点号"`
	SwVersion     uint32    `json:"sw_version" gorm:"comment:固件版本号"`
	ConfVersion   uint32    `json:"conf_version" gorm:"comment:配置版本号"`
	OnlineStatus  bool      `json:"online_status" gorm:"default:false;comment:在线状态"`
	LastCheckinAt *time.Time `json:"last_checkin_at" gorm:"comment:最后签到时间"`
	InstalledAt   *time.Time `json:"installed_at" gorm:"comment:安装日期"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Device) TableName() string {
	return "devices"
}
