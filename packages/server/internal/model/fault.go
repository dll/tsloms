package model

import "time"

// FaultRecord 故障记录表
// 记录设备上报的故障信息，同一设备同一 errCode 30分钟内去重
// 生命周期状态机：occurred(发生) → confirmed(确认) → dispatched(已派单) → resolved(已解决)
type FaultRecord struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	DeviceHwID   uint32     `json:"device_hw_id" gorm:"index;comment:设备硬件ID"`
	ErrCode      int8       `json:"err_code" gorm:"comment:错误码(-1至-14)"`
	FaultType    string     `json:"fault_type" gorm:"size:32;comment:故障类型分类"`
	FaultLevel   string     `json:"fault_level" gorm:"size:16;comment:故障等级(critical/normal)"`
	LedState     int8       `json:"led_state" gorm:"comment:故障时灯组状态"`
	CurrentR     uint16     `json:"current_r" gorm:"comment:红灯电流值"`
	CurrentY     uint16     `json:"current_y" gorm:"comment:黄灯电流值"`
	CurrentG     uint16     `json:"current_g" gorm:"comment:绿灯电流值"`
	FirstSeen    time.Time  `json:"first_seen" gorm:"comment:首次出现时间"`
	LastSeen     time.Time  `json:"last_seen" gorm:"comment:最后出现时间"`
	Status       string     `json:"status" gorm:"size:16;default:occurred;comment:状态(occurred/confirmed/dispatched/resolved)"`
	OwnerID      *uint      `json:"owner_id" gorm:"comment:负责人ID(故障确认人)"`
	RepairerID   *uint      `json:"repairer_id" gorm:"comment:维修人ID"`
	ConfirmedAt  *time.Time `json:"confirmed_at" gorm:"comment:确认时间"`
	DispatchedAt *time.Time `json:"dispatched_at" gorm:"comment:派单时间"`
	ResolvedAt   *time.Time `json:"resolved_at" gorm:"comment:解决时间"`
	WorkOrderID  *uint      `json:"work_order_id" gorm:"comment:关联工单ID"`

	// ---- 智能多源故障识别研判引擎(范围A)新增可空字段 ----
	// 全部可空/带缺省，兼容旧记录与前端 fault.ts 解析；只做加法不改既有列。
	Confidence        *float64   `json:"confidence" gorm:"comment:识别置信度(0-1)"`
	RecognitionSource string     `json:"recognition_source" gorm:"size:24;default:rule;comment:判定来源(rule/multi-source/case)"`
	RecognitionStatus string     `json:"recognition_status" gorm:"size:24;default:confirmed;comment:研判分流(confirmed/pending_review/filtered)"`
	IsFalsePositive   *bool      `json:"is_false_positive" gorm:"comment:是否被后续判定为误报"`
	EvidenceCount     int        `json:"evidence_count" gorm:"default:0;comment:参与研判的证据数"`
	LastEvaluationID  string     `json:"last_evaluation_id" gorm:"size:40;index;comment:末次研判批次号"`
	ReviewedAt        *time.Time `json:"reviewed_at" gorm:"comment:待确认复核通过时间"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// 识别研判分流状态常量（第 3 层判定分流结果）
const (
	// RecognitionConfirmed 高置信确认：直接落库，critical 自动派单
	RecognitionConfirmed = "confirmed"
	// RecognitionPendingReview 待确认：存疑/证据冲突，不自动派单，可被证据补充升级为确认
	RecognitionPendingReview = "pending_review"
	// RecognitionFiltered 误报过滤：仅记证据日志，不产生故障/工单
	RecognitionFiltered = "filtered"
)

// 判定来源常量
const (
	RecognitionSourceRule        = "rule"         // 确定性规则基座
	RecognitionSourceMultiSource = "multi-source" // 多源交叉验证融合
	RecognitionSourceCase        = "case"         // 案例库命中
)

// FaultRecognition 携带判定结果的结构（engine 输出，供业务落库）
type FaultRecognition struct {
	FaultType         string  // 判定故障类型
	FaultLevel        string  // 判定故障等级
	Confidence        float64 // 融合置信度 0-1
	RecognitionStatus string  // confirmed / pending_review / filtered
	RecognitionSource string  // rule / multi-source / case
	EvidenceCount     int     // 参与证据数
	EvaluationID      string  // 研判批次号
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
