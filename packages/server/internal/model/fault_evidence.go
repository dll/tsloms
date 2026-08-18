package model

import "time"

// FaultEvidence 多源证据表（范围A：智能多源故障识别研判引擎）
// 每起研判的证据来源明细：固件 errCode、电流/灯态、群众反映、手机举证、视频监控等。
// 被误报过滤(未落故障)的证据也保留，通过 evaluation_id 关联研判批次，保证 100% 可溯源。
type FaultEvidence struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	FaultID       *uint     `json:"fault_id" gorm:"index;comment:关联故障记录ID(被过滤的证据可为空)"`
	EvaluationID  string    `json:"evaluation_id" gorm:"size:40;index;comment:研判批次号(一次研判一组证据同批次)"`
	DeviceHwID    string    `json:"device_hw_id" gorm:"size:64;index:idx_evi_hw_time;comment:设备硬件ID(uuid字符串)"`
	SourceType    string    `json:"source_type" gorm:"size:24;index;comment:证据来源(firmware/current/led_state/citizen/photo_evidence/video_monitor)"`
	ErrCode       *int8     `json:"err_code" gorm:"comment:固件错误码(firmware类证据带)"`
	LedState      *int8     `json:"led_state" gorm:"comment:灯组状态(firmware类)"`
	CurrentR      *uint16   `json:"current_r" gorm:"comment:红灯电流值(firmware类)"`
	CurrentY      *uint16   `json:"current_y" gorm:"comment:黄灯电流值(firmware类)"`
	CurrentG      *uint16   `json:"current_g" gorm:"comment:绿灯电流值(firmware类)"`
	RawData       string    `json:"raw_data" gorm:"type:text;comment:原始报文/JSON/图片URL/文本"`
	RefMediaID    *uint     `json:"ref_media_id" gorm:"comment:关联device_media.id(举证/监控证据)"`
	RefFeedbackID *uint     `json:"ref_feedback_id" gorm:"comment:关联feedbacks.id(群众反映证据)"`
	CapturedAt    time.Time `json:"captured_at" gorm:"comment:证据发生时间(时间窗聚合依据)"`
	Confidence    float64   `json:"confidence" gorm:"default:0;comment:该证据对判定的贡献度0-1"`
	CreatedAt     time.Time `json:"created_at"`
}

// TableName 指定表名
func (FaultEvidence) TableName() string { return "fault_evidence" }

// 证据来源枚举常量
const (
	// EvSourceFirmware 固件 errCode 事件（主信号）
	EvSourceFirmware = "firmware"
	// EvSourceCurrent 电流信号（交叉校验）
	EvSourceCurrent = "current"
	// EvSourceLedState 灯态信号（交叉校验）
	EvSourceLedState = "led_state"
	// EvSourceCitizen 群众反映（辅助证据）
	EvSourceCitizen = "citizen"
	// EvSourcePhotoEvidence 手机举证图片/视频（辅助证据）
	EvSourcePhotoEvidence = "photo_evidence"
	// EvSourceVideoMonitor 视频监控报警（辅助证据，本阶段仅记录）
	EvSourceVideoMonitor = "video_monitor"
)

// 所有证据来源（供 /evidence/sources 枚举）
var EvidenceSourceTypes = []string{
	EvSourceFirmware, EvSourceCurrent, EvSourceLedState,
	EvSourceCitizen, EvSourcePhotoEvidence, EvSourceVideoMonitor,
}
