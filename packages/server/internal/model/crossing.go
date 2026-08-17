package model

import "time"

// Area 行政区划表（P0-4，支持 省→市→区县→街道→社区→道路 层级；道路可挂接路口）
// 对齐参考项目 a 的 areas 结构并扩展多级；树形通过 parent_id 递归表达同级顺序。
// 软删除可选；本表仅做「只增」，不破坏既有表。
type Area struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Code      string    `json:"code" gorm:"size:32;comment:区划编码(max 340103 式)"`
	Name      string    `json:"name" gorm:"size:64;index;comment:名称"`
	ParentID  *uint     `json:"parent_id" gorm:"index;comment:上级区划ID(空=顶级省)"`
	AreaType  string    `json:"area_type" gorm:"size:16;index;comment:层级(province/city/district/street/community/road)"`
	FullName  string    `json:"full_name" gorm:"size:255;comment:完整名称(如 安徽省合肥市瑶海区)"`
	AreaSort  int       `json:"area_sort" gorm:"default:0;comment:排序"`
	Remark    string    `json:"remark" gorm:"size:255;comment:备注"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Area) TableName() string { return "areas" }

// Area 层级常量
const (
	AreaProvince  = "province"  // 省
	AreaCity      = "city"      // 市
	AreaDistrict  = "district"  // 区县
	AreaStreet    = "street"    // 街道
	AreaCommunity = "community" // 社区
	AreaRoad      = "road"      // 道路
)

// AreaPath 路径（省→…→路）名称切片，供前端树/回显
type AreaPath struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Crossing 路口对象化表（P0-4/P0-5）
// 现状路口仅由设备 intersection 聚合得到（无独立表）；本表将路口对象化，
// 存：名称、行政区划外键(省/市/区/街道/社区/道路)、经纬度、状态（聚合缓存）。
// 与 devices 的关系：devices.crossing_id → crossings.id（一对多，可空）。
type Crossing struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	PointNo     string    `json:"point_no" gorm:"size:64;index;comment:点位编码"`
	Name        string    `json:"name" gorm:"size:128;index;comment:路口名称"`
	Type        string    `json:"type" gorm:"size:16;comment:路口类型(signal_cross_type:1直角/2卡口/3+多路口)"`
	ProvinceID  *uint     `json:"province_id" gorm:"comment:省ID"`
	CityID      *uint     `json:"city_id" gorm:"comment:市ID"`
	DistrictID  *uint     `json:"district_id" gorm:"comment:区县ID"`
	StreetID    *uint     `json:"street_id" gorm:"comment:街道ID"`
	CommunityID *uint     `json:"community_id" gorm:"comment:社区ID"`
	RoadID      *uint     `json:"road_id" gorm:"comment:道路ID"`
	RoadName    string    `json:"road_name" gorm:"size:128;comment:道路名称(冗余,便于直接展示)"`
	Lat         *float64  `json:"lat" gorm:"comment:纬度"`
	Lng         *float64  `json:"lng" gorm:"comment:经度"`
	Status      string    `json:"status" gorm:"size:16;default:normal;comment:聚合状态缓存(increase维护中/monitor监测中/offline离线/abnormal异常/flashing黄闪/normal正常)"`
	FaultRatio  float64   `json:"fault_ratio" gorm:"default:0;comment:故障比例缓存(故障设备/全部)"`
	GreenRatio  float64   `json:"green_ratio" gorm:"default:0;comment:绿灯正常比例缓存"`
	Remark      string    `json:"remark" gorm:"size:255;comment:备注"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Crossing) TableName() string { return "crossings" }

// Crossing 状态常量
const (
	CrossingStatusNormal   = "normal"   // 正常（全绿）
	CrossingStatusAbnormal = "abnormal" // 异常（故障比例非零）
	CrossingStatusOffline  = "offline"  // 离线/断电（全红）
	CrossingStatusMaintain = "maintain" // 维护中
	CrossingStatusFlashing = "flashing" // 黄闪
	CrossingStatusMonitor  = "monitor"  // 监测中
)

// ComputeCrossingStatus 依据故障比例与绿灯比例推导路口聚合状态。
// 规则（对齐需求分级着色）：
//   - 设备数=0 → offline? 实际返回 normal（无设备视为“无参考”）
//   - 全部故障/断电（fault_ratio>=1）→ offline（全部红）
//   - fault_ratio==0 且 green_ratio>0 → normal（全绿）
//   - fault_ratio>0 → abnormal（由绿→黄→红渐变，用比例区分）
func ComputeCrossingStatus(faultRatio, greenRatio float64) string {
	if faultRatio >= 1.0 {
		return CrossingStatusOffline
	}
	if faultRatio > 0 {
		return CrossingStatusAbnormal
	}
	return CrossingStatusNormal
}
