package model

import "time"

// FirmwarePackage 固件包表
// 记录信号灯设备 OTA 升级的固件版本包：文件存储、版本号、发布状态
type FirmwarePackage struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Version     string     `json:"version" gorm:"size:32;uniqueIndex;comment:固件版本号(如 v1.2.0)"`
	Major       uint32     `json:"major" gorm:"comment:主版本号"`
	Minor       uint32     `json:"minor" gorm:"comment:次版本号"`
	Build       uint32     `json:"build" gorm:"comment:构建号"`
	SwVersion   uint32     `json:"sw_version" gorm:"comment:对应设备固件位域值(0xMMmm..)"`
	FileName    string     `json:"file_name" gorm:"size:128;comment:固件文件名"`
	FilePath    string     `json:"file_path" gorm:"size:255;comment:固件存储路径"`
	Size        int64      `json:"size" gorm:"comment:文件大小(字节)"`
	MD5         string     `json:"md5" gorm:"size:32;comment:文件MD5校验"`
	Description string     `json:"description" gorm:"size:500;comment:升级说明/变更日志"`
	Published   bool       `json:"published" gorm:"default:false;comment:是否已发布(允许设备升级)"`
	PublishedAt *time.Time `json:"published_at" gorm:"comment:发布时间"`
	Uploader    string     `json:"uploader" gorm:"size:64;comment:上传人"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (FirmwarePackage) TableName() string {
	return "firmware_packages"
}

// FirmwareUpgradeRecord 设备固件升级记录表
// 记录某设备对某固件包的升级任务与结果
type FirmwareUpgradeRecord struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	FirmwareID  uint       `json:"firmware_id" gorm:"index;comment:固件包ID"`
	DeviceHwID  uint32     `json:"device_hw_id" gorm:"index;comment:设备硬件ID"`
	TargetVer   string     `json:"target_version" gorm:"size:32;comment:目标固件版本"`
	Status      string     `json:"status" gorm:"size:24;default:pending;comment:状态(pending/upgrading/success/failed)"`
	ErrorMsg    string     `json:"error_msg" gorm:"size:500;comment:失败原因"`
	StartedAt   *time.Time `json:"started_at" gorm:"comment:升级开始时间"`
	FinishedAt  *time.Time `json:"finished_at" gorm:"comment:升级结束时间"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (FirmwareUpgradeRecord) TableName() string {
	return "firmware_upgrade_records"
}

// 升级状态常量
const (
	FirmwareUpgradePending    = "pending"    // 等待升级
	FirmwareUpgradeUpgrading  = "upgrading"  // 升级中
	FirmwareUpgradeSuccess    = "success"    // 升级成功
	FirmwareUpgradeFailed     = "failed"     // 升级失败
)
