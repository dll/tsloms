package model

import "time"

// ============================================================================
// P1 自动巡检：巡检任务 / 巡检记录 / 巡检排行
// ----------------------------------------------------------------------------
// 三种巡检模式 + 信号灯自检 + AI 硬件：
//   - area     : 按空间区域（区县/街道）筛选设备
//   - street   : 按街道筛选设备
//   - random   : 随机抽检 target_count 台设备
//   - selfcheck: 信号灯自检（采集灯态/errCode 判定 normal/abnormal）
//   - ai       : 从 AI 预测(AIPrediction)高危设备接入硬件数据
//
// 全部只做加法，不改既有表；表独立命名 patrol_*，不与 /faults、/devices 冲突。
// ============================================================================

// PatrolTask 巡检任务表
type PatrolTask struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Name        string     `json:"name" gorm:"size:128;comment:任务名称"`
	Mode        string     `json:"mode" gorm:"size:16;index;comment:巡检模式(area/street/random/selfcheck/ai)"`
	AreaID      *uint      `json:"area_id" gorm:"index;comment:空间区域ID(区县/区域)"`
	StreetID    *uint      `json:"street_id" gorm:"index;comment:街道ID"`
	TimeWindow  string     `json:"time_window" gorm:"size:32;comment:巡检时段窗口(如 08:00-10:00,可空)"`
	TargetCount int        `json:"target_count" gorm:"default:0;comment:抽检数量(random模式用)"`
	Status      string     `json:"status" gorm:"size:16;default:planned;index;comment:状态(planned/running/done)"`
	AssigneeID  *uint      `json:"assignee_id" gorm:"index;comment:指派人(执行人)ID"`
	CreatedBy   uint       `json:"created_by" gorm:"index;comment:创建人ID"`
	LastRunAt   *time.Time `json:"last_run_at" gorm:"comment:最后执行时间"`
	RunCount    int        `json:"run_count" gorm:"default:0;comment:已执行次数"`
	Remark      string     `json:"remark" gorm:"size:255;comment:备注"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (PatrolTask) TableName() string { return "patrol_tasks" }

// PatrolTask 模式常量
const (
	PatrolModeArea      = "area"      // 空间区域（区县/区域）
	PatrolModeStreet    = "street"    // 街道
	PatrolModeRandom    = "random"    // 随机抽检
	PatrolModeSelfCheck = "selfcheck" // 信号灯自检
	PatrolModeAI        = "ai"        // AI 硬件接入
)

// PatrolTask 状态常量
const (
	PatrolStatusPlanned = "planned" // 已计划（未执行）
	PatrolStatusRunning = "running" // 执行中
	PatrolStatusDone    = "done"    // 已完成
)

// PatrolRecord 巡检记录表
type PatrolRecord struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	TaskID          *uint     `json:"task_id" gorm:"index;comment:巡检任务ID"`
	DeviceID        *uint     `json:"device_id" gorm:"index;comment:设备表ID(可空)"`
	DeviceHwID      string    `json:"device_hw_id" gorm:"size:64;index;comment:设备硬件ID(uuid字符串)"`
	CrossingID      *uint     `json:"crossing_id" gorm:"index;comment:路口ID"`
	PatrolType      string    `json:"patrol_type" gorm:"size:16;index;comment:巡检类型(area/street/random/selfcheck/ai)"`
	CheckResult     string    `json:"check_result" gorm:"size:16;index;comment:自检判定(normal/abnormal)"`
	CheckDetail     string    `json:"check_detail" gorm:"size:512;comment:巡检/自检详情说明"`
	SelfCheckResult string    `json:"selfcheck_result" gorm:"type:text;comment:自检采集数据JSON(灯态/errCode/电流)"`
	Evidences       string    `json:"evidences" gorm:"type:text;comment:证据JSON(巡检照片/来源等)"`
	PatrolBy        string    `json:"patrol_by" gorm:"size:64;index;comment:巡检人(姓名/账号)"`
	PatrolAt        time.Time `json:"patrol_at" gorm:"index;comment:巡检时间"`
	Lat             *float64  `json:"lat" gorm:"comment:纬度"`
	Lng             *float64  `json:"lng" gorm:"comment:经度"`
	CreatedAt       time.Time `json:"created_at"`
}

// TableName 指定表名
func (PatrolRecord) TableName() string { return "patrol_records" }

// PatrolRecord 判定结果常量
const (
	PatrolResultNormal   = "normal"   // 正常
	PatrolResultAbnormal = "abnormal" // 异常
)

// PatrolRankingItem 巡检排行聚合条目（对齐 a /inspectRanking）
// 按巡检人/设备聚合：巡检人次、异常数、异常率、最后巡检时间。
type PatrolRankingItem struct {
	Key          string     `json:"key"`            // 聚合维度值（巡检人或设备）
	PatrolCount  int        `json:"patrol_count"`   // 巡检人次
	AbnormalCnt  int        `json:"abnormal_count"` // 异常数
	AbnormalRate float64    `json:"abnormal_rate"`  // 异常率(0-1)
	LastPatrolAt *time.Time `json:"last_patrol_at"` // 最后巡检时间
}
