package model

import "time"

// SmsCode 短信/一次性验证码表（P0-2 认证改造）
// 用于手机号+验证码登录的验证码记录：手机号、验证码、用途、过期时间、校验次数、已用标记。
// 首版支持「本地模拟」通道（Console 输出 + 固定测试码 devcode 供开发），
// 生产未配置短信服务商时自动降级 Console，绝不阻塞登录闭环。
type SmsCode struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	Phone      string     `json:"phone" gorm:"size:20;index;comment:手机号"`
	Code       string     `json:"code" gorm:"size:16;comment:验证码"`
	Biz        string     `json:"biz" gorm:"size:16;default:auth;index;comment:用途(auth登录/verify绑定等)"`
	ExpiresAt  time.Time  `json:"expires_at" gorm:"comment:过期时间"`
	Used       bool       `json:"used" gorm:"default:false;comment:是否已使用（一次性）"`
	VerifyCnt  int        `json:"verify_cnt" gorm:"default:0;comment:已校验次数"`
	CreatedAt  time.Time  `json:"created_at"`
	VerifiedAt *time.Time `json:"verified_at" gorm:"comment:校验通过时间"`
}

// TableName 指定表名
func (SmsCode) TableName() string { return "sms_codes" }

// 验证码用途常量
const (
	SmsBizAuth   = "auth"   // 登录
	SmsBizVerify = "verify" // 绑定/换绑手机号
)

// SmsCodeMaxVerify 单条验证码允许的最大校验次数（防御暴力试码）
const SmsCodeMaxVerify = 5

// SmsCodeTTLMinutes 验证码有效时长（分钟）
const SmsCodeTTLMinutes = 10

// SmsCodeLen 验证码长度
const SmsCodeLen = 6
