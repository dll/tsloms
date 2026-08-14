package model

import "time"

// DeviceMedia 设备媒体表
// 承载三种视频/图片能力：
//   - 举证（手机短视频/图片，维修现场取证）
//   - 监控（路灯监控，RTSP 实况或云存储 URL）
//   - 时间视频（监控自动截取的短视频片段，辅助维修判断）
type DeviceMedia struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	DeviceHwID  uint32    `json:"device_hw_id" gorm:"index;comment:设备硬件ID"`
	MediaType   string    `json:"media_type" gorm:"size:16;comment:媒体类型(evidence/monitoring/timelapse)"`
	Category    string    `json:"category" gorm:"size:32;comment:类别(photo/video)"`
	Title       string    `json:"title" gorm:"size:128;comment:标题"`
	Source      string    `json:"source" gorm:"size:16;comment:来源(upload/rtsp/url)"`
	URL         string    `json:"url" gorm:"size:512;comment:媒体地址(本地文件/云URL/RTSP)"`
	CompatibleURL string  `json:"compatible_url" gorm:"size:512;comment:兼容播放地址(如RTSP转HLS)"`
	Thumbnail   string    `json:"thumbnail" gorm:"size:512;comment:封面图URL"`
	Duration    int       `json:"duration" gorm:"comment:时长(秒,视频)"`
	Note        string    `json:"note" gorm:"type:text;comment:备注"`
	UploadedBy  string    `json:"uploaded_by" gorm:"size:64;comment:上传人"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 指定表名
func (DeviceMedia) TableName() string {
	return "device_media"
}

// 媒体类型常量
const (
	MediaEvidence    = "evidence"    // 举证（手机短视频/图片）
	MediaMonitoring  = "monitoring"  // 路灯监控（RTSP/云URL）
	MediaTimelapse   = "timelapse"   // 时间视频（监控自动截取片段）
	MediaPhoto       = "photo"       // 图片
	MediaVideo       = "video"       // 视频
	MediaSourceUpload = "upload"     // 手机上传
	MediaSourceRTSP  = "rtsp"        // RTSP 实况
	MediaSourceURL   = "url"         // 云存储 URL
)
