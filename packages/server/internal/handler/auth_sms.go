package handler

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
	"gorm.io/gorm"
)

// ============================================================================
// P0-2 认证改造：手机号 + 验证码登录（双通道向后兼容）
// ----------------------------------------------------------------------------
// - POST /auth/sms-code       发送验证码（可插拔通道）
// - POST /auth/login          多态登录：login_type=pwd（旧 用户名+密码）/ sms（手机号+验证码）
//
// 短信通道（SmsSender）：可插拔。
//   - console:   验证码打印到服务端日志（开发/演示；生产未配置服务商时自动降级到这里）
//   - devcode:   使用固定测试码（SMS_DEV_CODE，开发联调用）
//   - 其它/空:   生产未配置真实 SMS 服务商时一律降级 console，绝不阻塞登录闭环。
//   预留真实 SMS 服务商接口：实现 SmsSender 接口并在此处注册即可扩展，不影响既有闭环。
// ============================================================================

// SmsSender 短信发送抽象接口（可插拔通道；预留真实短信服务商实现点）
type SmsSender interface {
	// ProviderName 服务商标识（console/devcode/aliyun/...）
	ProviderName() string
	// Send 发送验证码。返回验证码明文（如需本地模拟/测试码）。
	// NOTE: 真实生产通道不应把明文返回给业务（仅需下发），此处为本地通道约定。
	Send(phone, code string) error
}

// consoleSmsSender 本地 Console 通道：仅打印验证码到日志（开发/降级保证）。
type consoleSmsSender struct{}

func (consoleSmsSender) ProviderName() string { return "console" }
func (consoleSmsSender) Send(phone, code string) error {
	// 日志打印（生产降级通道亦可用；不阻塞业务）
	fmt.Printf("[SMS][console] 手机号 %s 验证码: %s (本地模拟通道)\n", phone, code)
	return nil
}

// devCodeSmsSender 开发固定测试码通道：统一返回配置的测试码。
type devCodeSmsSender struct{ code string }

func (devCodeSmsSender) ProviderName() string { return "devcode" }
func (d devCodeSmsSender) Send(phone, code string) error {
	// devcode 通道：实际下发固定测试码（忽略随机 code）
	fmt.Printf("[SMS][devcode] 手机号 %s 使用固定测试码: %s (开发通道)\n", phone, d.code)
	return nil
}

// smsSender 当前选中的发送通道（进程内缓存）
var smsSender SmsSender

// resolveSmsSender 依据配置解析发送通道；默认 console（保证必可发送）。
func resolveSmsSender() SmsSender {
	if smsSender != nil {
		return smsSender
	}
	cfg := config.Get()
	switch cfg.SmsProvider {
	case "devcode":
		code := cfg.SmsDevCode
		if code == "" {
			code = "123456"
		}
		smsSender = devCodeSmsSender{code: code}
	default:
		// console（含未配置服务商 → 生产自动降级，绝不阻塞登录闭环）
		smsSender = consoleSmsSender{}
	}
	return smsSender
}

// randomSmsCode 生成 n 位数字验证码（密码学安全随机）
func randomSmsCode(n int) string {
	if n <= 0 {
		n = model.SmsCodeLen
	}
	sb := make([]byte, n)
	for i := 0; i < n; i++ {
		d, _ := rand.Int(rand.Reader, big.NewInt(10))
		sb[i] = byte('0' + d.Int64())
	}
	return string(sb)
}

// GenerateSmsCode 生成并下发一条验证码（落库 sms_codes）；返回应下发的验证码明文。
// 开发/测试可直接调用（传入 *gorm.DB）。
func GenerateSmsCode(db *gorm.DB, phone, biz string) (string, error) {
	n := model.SmsCodeLen
	code := randomSmsCode(n)
	// devcode 通道优先用固定测试码
	if s := resolveSmsSender(); s.ProviderName() == "devcode" {
		if dc := s.(devCodeSmsSender).code; dc != "" {
			code = dc
		}
	}
	ttl := config.Get().SmsCodeTTL
	if ttl <= 0 {
		ttl = model.SmsCodeTTLMinutes
	}
	rec := model.SmsCode{
		Phone:     phone,
		Code:      code,
		Biz:       biz,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Minute),
	}
	if err := db.Create(&rec).Error; err != nil {
		return "", err
	}
	// 调用发送通道（console/devcode 均只做本地输出/占位；真实服务商在此接入）
	if err := resolveSmsSender().Send(phone, code); err != nil {
		// 发送失败不阻塞（降级通道无实际发送失败场景），但记录
		return "", err
	}
	return code, nil
}

// validPhone 简单校验中国大陆手机号（1 开头，11 位数字；宽松校验便于演示）
func validPhone(phone string) bool {
	if len(phone) != 11 {
		return false
	}
	for i := 0; i < len(phone); i++ {
		if phone[i] < '0' || phone[i] > '9' {
			return false
		}
	}
	return phone[0] == '1'
}

// SendSmsCode POST /auth/sms-code
// 请求: {phone: "13xxxxxxxxx", biz?: "auth", type?: "login"}
// 响应: { message, debug?: { code }（仅 console/devcode 通道返回，便于开发联调）}
// 公开接口（无需登录）。生产未配置真实服务商时自动降级 Console，绝不阻塞。
func SendSmsCode(c *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
		Biz   string `json:"biz"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请输入手机号")
		return
	}
	req.Phone = trimPhone(req.Phone)
	if !validPhone(req.Phone) {
		badRequest(c, "手机号格式不正确")
		return
	}
	if req.Biz == "" {
		req.Biz = model.SmsBizAuth
	}

	// 可插拔通道；devcode/console 情况下把验证码回显给前端（dev 联调用）
	sender := resolveSmsSender()
	debug := gin.H{}
	if sender.ProviderName() == "console" || sender.ProviderName() == "devcode" {
		// 本地通道：返回验证码便于前端演示（生产走真实服务商时不返回明文）
		ttl := config.Get().SmsCodeTTL
		if ttl <= 0 {
			ttl = model.SmsCodeTTLMinutes
		}
		debug["channel"] = sender.ProviderName()
		debug["ttl_minutes"] = ttl
		debug["dev"] = true
	}

	code, err := GenerateSmsCode(model.DB, req.Phone, req.Biz)
	if err != nil {
		serverError(c, err)
		return
	}
	if debug["dev"] == true {
		debug["code"] = code // 本地通道回显验证码（仅开发，勿在真实生产启用真实通道）
	}
	ok(c, gin.H{"message": "验证码已发送", "debug": debug})
}

// verifySmsCode 校验验证码（一次性）：查询同手机号+用途最新未用且未过期记录，比对 code。
// 带上限的校验次数（SmsCodeMaxVerify）防御暴力试码；成功后标记已用。
func verifySmsCode(phone, code, biz string) bool {
	if phone == "" || code == "" {
		return false
	}
	now := time.Now()
	var rec model.SmsCode
	err := model.DB.Where("phone = ? AND biz = ? AND used = ? AND expires_at > ?",
		phone, biz, false, now).
		Order("id DESC").First(&rec).Error
	if err != nil {
		return false
	}
	// 校验次数防御
	if rec.VerifyCnt >= model.SmsCodeMaxVerify {
		return false
	}
	if rec.Code != code {
		// 记录失败次数
		model.DB.Model(&rec).Update("verify_cnt", rec.VerifyCnt+1)
		return false
	}
	// 校验通过：标记已用（一次性），并记录通过时间
	now2 := time.Now()
	model.DB.Model(&rec).Updates(map[string]interface{}{"used": true, "verify_cnt": rec.VerifyCnt + 1, "verified_at": &now2})
	return true
}

// findUserByLogin 按手机号登录账号或用户名定位用户（旧账号 username 兼容）。
func findUserByLogin(login string) *model.User {
	if login == "" {
		return nil
	}
	// 优先按手机号登录账号
	var u model.User
	if err := model.DB.Where("phone_login = ?", login).First(&u).Error; err == nil {
		return &u
	}
	// 兼容：username 的既有账号
	if err := model.DB.Where("username = ?", login).First(&u).Error; err == nil {
		return &u
	}
	return nil
}

// phoneLogin 手机号+验证码登录分支。
func phoneLogin(c *gin.Context, phone string, code string) {
	if !validPhone(phone) {
		unauthorized(c, "手机号格式不正确")
		return
	}
	if !verifySmsCode(phone, code, model.SmsBizAuth) {
		unauthorized(c, "验证码错误或已过期")
		return
	}
	// 定位用户：绑定了 phone_login 的手机号
	var user model.User
	// 优先 phone_login
	err := model.DB.Where("phone_login = ?", phone).First(&user).Error
	if err != nil {
		// 未绑定：尝试 username 或 phone 字段（若已有账号的手机号相同则绑定使用）
		err2 := model.DB.Where("username = ? OR phone = ?", phone, phone).First(&user).Error
		if err2 != nil {
			unauthorized(c, "该手机号未注册账号")
			return
		}
		// 首次用手机号登录的既有账号：回填 phone_login 以便后续识别
		model.DB.Model(&user).Updates(map[string]interface{}{"phone_login": phone, "phone_verified": true})
	}
	if user.Status == model.UserStatusDisabled {
		unauthorized(c, "账号已停用，请联系管理员")
		return
	}
	// 更新最后登录时间
	now := time.Now()
	model.DB.Model(&user).Update("last_login_at", now)

	cfg := config.Get()
	token, err := issueToken(&user, cfg.JWTSecret)
	if err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":             user.ID,
			"username":       user.Username,
			"role":           user.Role,
			"real_name":      user.RealName,
			"phone":          user.Phone,
			"phone_login":    user.PhoneLogin,
			"phone_verified": user.PhoneVerified,
			"email":          user.Email,
			"department_id":  user.DepartmentID,
			"status":         user.Status,
		},
		"enabled_modules": EnabledModuleList(),
	})
	c.Set("op_username", user.Username)
}

// trimPhone 去除手机号首尾空白与常见分隔符
func trimPhone(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == ' ' || ch == '-' || ch == '+' || ch == '\t' || ch == '\n' {
			continue
		}
		out = append(out, ch)
	}
	if len(out) > 11 {
		// 可能带 +86 前缀：去掉前 3 位
		out = out[len(out)-11:]
	}
	return string(out)
}
