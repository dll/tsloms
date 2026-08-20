package model

import "time"

// Device 设备表
// 记录信号灯监控设备的台账信息，hw_id 为出厂唯一硬件 ID
// 路口维度：intersection 为路口名称，lat/lng 为设备经纬度（用于地图打点）
type Device struct {
	ID           uint   `json:"id" gorm:"primaryKey"`
	HwID         string `json:"hw_id" gorm:"size:64;uniqueIndex;comment:设备硬件ID(uuid字符串,出厂唯一)"`
	Intersection string `json:"intersection" gorm:"size:128;index;comment:路口位置描述"`
	// P0-4 路口/行政区划挂接：均可空，只增不删，未挂接线旧设备不受影响
	CrossingID   *uint    `json:"crossing_id" gorm:"index;comment:所属路口ID(crossings.id,可空)"`
	ProvinceID   *uint    `json:"province_id" gorm:"comment:省ID"`
	CityID       *uint    `json:"city_id" gorm:"comment:市ID"`
	DistrictID   *uint    `json:"district_id" gorm:"comment:区县ID"`
	StreetID     *uint    `json:"street_id" gorm:"comment:街道ID"`
	CommunityID  *uint    `json:"community_id" gorm:"comment:社区ID"`
	RoadID       *uint    `json:"road_id" gorm:"comment:道路ID"`
	RoadName     string   `json:"road_name" gorm:"size:128;comment:道路名称(冗余)"`
	Lat          *float64 `json:"lat" gorm:"comment:纬度"`
	Lng          *float64 `json:"lng" gorm:"comment:经度"`
	NetworkCode  int      `json:"network_code" gorm:"comment:网络号"`
	StationCode  int      `json:"station_code" gorm:"comment:站点号"`
	SwVersion    uint32   `json:"sw_version" gorm:"comment:固件版本号"`
	ConfVersion  uint32   `json:"conf_version" gorm:"comment:配置版本号"`
	OnlineStatus bool     `json:"online_status" gorm:"default:false;comment:在线状态"`
	// 设备生命周期与接入状态：手工新增先预登记，首次 MQTT 上报后转为已接入。
	LifecycleStatus    string     `json:"lifecycle_status" gorm:"size:16;default:active;index;comment:生命周期(active/retired)"`
	AccessStatus       string     `json:"access_status" gorm:"size:16;default:never;index;comment:接入状态(never/accessed/offline)"`
	FirstAccessAt      *time.Time `json:"first_access_at" gorm:"comment:首次有效接入时间"`
	RetiredAt          *time.Time `json:"retired_at" gorm:"comment:报废时间"`
	RetiredReason      string     `json:"retired_reason" gorm:"size:255;comment:报废原因"`
	RegistrationSource string     `json:"registration_source" gorm:"size:16;default:manual;comment:建账来源(manual/mqtt_auto)"`
	IsWatched          bool       `json:"is_watched" gorm:"default:false;comment:是否关注(锁定/可能故障)"`
	LastCheckinAt      *time.Time `json:"last_checkin_at" gorm:"comment:最后签到时间"`
	InstalledAt        *time.Time `json:"installed_at" gorm:"comment:安装日期"`
	// 设备资料（照片/说明书/维修手册）：URL 可为外链或内部上传地址（/media/...），Name 为上传原文件名（用于阅读/下载展示）
	Photo            string `json:"photo" gorm:"size:500;comment:设备照片URL"`
	ManualUrl        string `json:"manual_url" gorm:"size:500;comment:说明书链接(外链或上传)"`
	ManualName       string `json:"manual_name" gorm:"size:255;comment:说明书文件名"`
	RepairManualUrl  string `json:"repair_manual_url" gorm:"size:500;comment:维修手册链接(外链或上传)"`
	RepairManualName string `json:"repair_manual_name" gorm:"size:255;comment:维修手册文件名"`
	// 对齐参考项目 a 的设备字段（a 的 equipment：func信号灯功能 / cx朝向 / fx方向 / batch批次 / remark备注）
	Func        string    `json:"func" gorm:"size:32;comment:信号灯功能(灯组类型,如直行/左转/右转)"`
	Orientation string    `json:"orientation" gorm:"size:16;comment:朝向(cx,如南/北/东/西/东南等)"`
	Direction   string    `json:"direction" gorm:"size:16;comment:方向(fx)"`
	Batch       string    `json:"batch" gorm:"size:32;comment:批次"`
	Remark      string    `json:"remark" gorm:"size:255;comment:备注"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Device) TableName() string {
	return "devices"
}

// 固件版本号位域解码（协议 PDF P6）
// bit[31:28]=major, bit[27:24]=minor, bit[23:18]=year(2000+n),
// bit[17:14]=month, bit[13:8]=day, bit[7:0]=build#
type SwVerInfo struct {
	Major uint32 `json:"major"`
	Minor uint32 `json:"minor"`
	Year  uint32 `json:"year"`
	Month uint32 `json:"month"`
	Day   uint32 `json:"day"`
	Build uint32 `json:"build"`
	Raw   uint32 `json:"raw"`
}

// DecodeSwVer 解码固件版本号
func DecodeSwVer(v uint32) SwVerInfo {
	return SwVerInfo{
		Major: (v >> 28) & 0xF,
		Minor: (v >> 24) & 0xF,
		Year:  (v >> 18) & 0x3F, // 6 bit，实际 2000+n
		Month: (v >> 14) & 0xF,
		Day:   (v >> 8) & 0x3F,
		Build: v & 0xFF,
		Raw:   v,
	}
}

// 配置版本号解码（协议 PDF P8）：0xYYMMDDnn
// YY=年 MM=月 DD=日 nn=当日版本
type ConfVerInfo struct {
	Year  uint32 `json:"year"`
	Month uint32 `json:"month"`
	Day   uint32 `json:"day"`
	Build uint32 `json:"build"`
	Raw   uint32 `json:"raw"`
}

// DecodeConfVer 解码配置版本号 0xYYMMDDnn
func DecodeConfVer(v uint32) ConfVerInfo {
	return ConfVerInfo{
		Year:  (v >> 24) & 0xFF,
		Month: (v >> 16) & 0xFF,
		Day:   (v >> 8) & 0xFF,
		Build: v & 0xFF,
		Raw:   v,
	}
}
