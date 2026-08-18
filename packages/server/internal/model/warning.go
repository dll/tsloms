package model

import "time"

// Warning 预警记录表（P0-3 预警管理，对齐参考项目 a 的 signal_warning）
// 预警是独立于 FaultRecord（故障识别-派单链路）的「前置/轻量通知」记录，
// 可源于故障(fault)、MQTT 上报、信号灯自检、人工 等。与故障/工单并存：
//   - 故障 = 已确认需处置并派单；预警 = 轻量通知记录，可忽略（处理）或转工单。
//
// 转工单后回填 WorkOrderID 并置 Status=transferred（已转）。
type Warning struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	DeviceHwID    string     `json:"device_hw_id" gorm:"size:64;index;comment:设备硬件ID(uuid字符串)"`
	CrossingID    *uint      `json:"crossing_id" gorm:"index;comment:路口ID(可空)"`
	EquipmentUUID string     `json:"equipment_uuid" gorm:"size:64;comment:冗余设备/信号机地址码"`
	WarningCode   int        `json:"warning_code" gorm:"index;comment:告警内容码(对齐识别引擎errCode -1~-14, 0=正常)"`
	WarningLabel  string     `json:"warning_label" gorm:"size:64;comment:告警文案"`
	Level         string     `json:"level" gorm:"size:16;default:warning;index;comment:级别(critical/warning/info)"`
	Func          string     `json:"func" gorm:"size:64;comment:相位/功能方向"`
	Source        string     `json:"source" gorm:"size:24;default:fault;index;comment:来源(fault/mqtt/selfcheck/manual)"`
	DealState     string     `json:"deal_state" gorm:"size:16;default:unhandled;index;comment:处理状态(unhandled/ignored/resolved)"`
	Status        string     `json:"status" gorm:"size:16;default:untransferred;index;comment:工单状态(untransferred/transferred)"`
	FaultID       *uint      `json:"fault_id" gorm:"index;comment:来源故障ID(若源自故障)"`
	WorkOrderID   *uint      `json:"work_order_id" gorm:"index;comment:转单后关联工单ID"`
	IgnoreReason  string     `json:"ignore_reason" gorm:"size:255;comment:忽略原因"`
	OccurredAt    time.Time  `json:"occurred_at" gorm:"index;comment:告警发生时间"`
	ResolvedAt    *time.Time `json:"resolved_at" gorm:"comment:处理/忽略时间"`
	Remark        string     `json:"remark" gorm:"size:255;comment:备注"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (Warning) TableName() string { return "warnings" }

// Warning 级别常量
const (
	WarningLevelCritical = "critical" // 严重（影响通行，如全灭/断电）
	WarningLevelWarning  = "warning"  // 一般
	WarningLevelInfo     = "info"     // 提示
)

// Warning 处理状态常量
const (
	WarningDealUnhandled = "unhandled" // 未处理
	WarningDealIgnored   = "ignored"   // 已忽略
	WarningDealResolved  = "resolved"  // 已解决/已转工单
)

// Warning 工单状态常量
const (
	WarningUntransferred = "untransferred" // 未转工单
	WarningTransferred   = "transferred"   // 已转工单
)

// Warning 来源常量
const (
	WarningSourceFault     = "fault"     // 源自故障记录
	WarningSourceMQTT      = "mqtt"      // MQTT 实时上报
	WarningSourceSelfCheck = "selfcheck" // 信号灯自检
	WarningSourceManual    = "manual"    // 人工
)

// WarningRule 预警配置/忽略规则表（P0-3，对齐参考项目 a 的 signal_warning_config）
// 按路口/设备/告警类型/级别 配置「忽略」规则；生效模式支持永久与时间段。
// 全部以可空字段表达「all=全部」默认语义，只做加法，不改既有表。
type WarningRule struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Name          string    `json:"name" gorm:"size:64;comment:规则名称"`
	CrossingID    *uint     `json:"crossing_id" gorm:"index;comment:路口ID(空=全部路口)"`
	DeviceHwID    *string   `json:"device_hw_id" gorm:"size:64;index;comment:设备硬件ID(uuid,空=全部设备)"`
	WarningCode   *int      `json:"warning_code" gorm:"index;comment:告警内容码(空=全部告警)"`
	Level         string    `json:"level" gorm:"size:16;comment:级别(空=全部级别)"`
	EffectiveType string    `json:"effective_type" gorm:"size:16;default:permanent;comment:生效模式(permanent/period)"`
	StartTime     string    `json:"start_time" gorm:"size:8;comment:生效开始时间(HH:mm, period模式)"`
	EndTime       string    `json:"end_time" gorm:"size:8;comment:生效结束时间(HH:mm, period模式)"`
	Action        string    `json:"action" gorm:"size:16;default:ignore;comment:动作(ignore 忽略)"`
	Enabled       bool      `json:"enabled" gorm:"default:true;comment:是否启用"`
	Remark        string    `json:"remark" gorm:"size:255;comment:备注"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName 指定表名
func (WarningRule) TableName() string { return "warning_rules" }

// WarningRule 生效模式常量
const (
	RuleEffectivePermanent = "permanent" // 永久
	RuleEffectivePeriod    = "period"    // 时间段
)

// WarningRule 动作常量
const (
	RuleActionIgnore = "ignore" // 忽略
)

// WarningRuleMatches 判断某条预警是否命中该规则（用于自动忽略判定）
// 规则启用、动作=ignore，且各维度匹配（nil/空=该维度全部命中）。
// period 模式：当前时间落在 [StartTime, EndTime)（HH:mm）内。
func (r *WarningRule) Matches(w *Warning) bool {
	if r == nil || w == nil {
		return false
	}
	if !r.Enabled || r.Action != RuleActionIgnore {
		return false
	}
	if r.CrossingID != nil && w.CrossingID == nil {
		// 规则限定路口，但预警无路口 → 不匹配
		return false
	}
	if r.CrossingID != nil && w.CrossingID != nil && *r.CrossingID != *w.CrossingID {
		return false
	}
	if r.DeviceHwID != nil && w.DeviceHwID != *r.DeviceHwID {
		return false
	}
	if r.WarningCode != nil && w.WarningCode != *r.WarningCode {
		return false
	}
	if r.Level != "" && w.Level != r.Level {
		return false
	}
	if r.EffectiveType == RuleEffectivePeriod {
		if !inTimeWindow(r.StartTime, r.EndTime) {
			return false
		}
	}
	return true
}

// inTimeWindow 判断当前时间(HH:mm)是否落在 [start,end) 窗口内（跨天支持）
func inTimeWindow(start, end string) bool {
	now := time.Now()
	hm := now.Format("15:04")
	if start == "" && end == "" {
		return true
	}
	if start == "" {
		return hm <= end
	}
	if end <= start {
		// 跨天窗口，例如 22:00 ~ 02:00
		return hm >= start || hm <= end
	}
	return hm >= start && hm < end
}
