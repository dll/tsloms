// Package license —— 授权/试用/防破解核心（仅供超管管理与模块拦截读取）
//
// 设计目标（用户要求）：
//   - 核心功能试用 100 天，可选功能试用 30 天；到期锁定，须超管授权解锁。
//   - 保密、难以被破解：授权状态用 Ed25519 签名（供应方私钥离线签名，服务器仅存公钥验签），
//     篡改授权内容/延长有效期即签名失效；并做系统时间回拨检测（记录最近校验时间，回拨即视为失效）。
//   - 简化操作：超管登录后一个按钮解锁 / 或输入供应方签发的授权码。
//   - 防死锁：核心到期后，业务功能锁定，但"超管登录 + 授权管理"始终可用（否则无法解锁）。
//   - 校验无副作用、纯函数化，便于单元测试。
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// 试用天数
const (
	TrialDaysCore     = 100 // 核心功能试用天数
	TrialDaysOptional = 30  // 可选功能试用天数
)

// 模块 key（与 handler/module.go 的 ModuleVideo/ModuleInventory/... 对齐，超管授权对象）
const (
	KeyVideo        = "video"
	KeyInventory    = "inventory"
	KeyPurchase     = "purchase"
	KeyExpense      = "expense"
	KeySupplier     = "supplier"
	KeyAI           = "ai"
	KeyDispatch     = "dispatch"
	KeyNotification = "notification"
	KeyPatrol       = "patrol"
)

// supplierPublicKeyB64 供应方 Ed25519 公钥（base64url）。服务器仅内嵌公钥用于验签，私钥由供应方离线工具持有，
// 不入库、不随服务器部署——篡改授权码/延长有效期均因签名不符而失效（防破解核心）。
const supplierPublicKeyB64 = "_Ilp6BWDR58wY3w9rsnbmY9Qy_PRPU2ltPsHpOA9-gs"

// cachedPublicKey 进程内缓存解码后的公钥
var cachedPublicKey ed25519.PublicKey

// SetPublicKeyForTest 仅测试专用：覆盖验签公钥，使测试可用独立测试密钥对（与生产私钥解耦）。
// 生产代码绝不调用本函数；不调用时验签仍使用默认生产公钥 supplierPublicKeyB64。
// 生产验签/授权机制不受影响。
func SetPublicKeyForTest(pub ed25519.PublicKey) {
	if pub == nil || len(pub) != ed25519.PublicKeySize {
		cachedPublicKey = nil
		return
	}
	cachedPublicKey = pub
}

// PublicKey 返回用于验签的 Ed25519 公钥；未配置/解码失败时返回 nil。
func PublicKey() ed25519.PublicKey {
	if cachedPublicKey != nil {
		return cachedPublicKey
	}
	b, err := base64.RawURLEncoding.DecodeString(supplierPublicKeyB64)
	if err != nil {
		return nil
	}
	if len(b) != ed25519.PublicKeySize {
		return nil
	}
	cachedPublicKey = ed25519.PublicKey(b)
	return cachedPublicKey
}

// OptionalModuleKeys 可选（试用授权）模块；核心模块恒启，不受试用锁定。
var OptionalModuleKeys = []string{
	KeyVideo, KeyInventory, KeyPurchase, KeyExpense, KeySupplier,
	KeyAI, KeyDispatch, KeyNotification, KeyPatrol,
}

// ParseUnlockCode 解析"超管授权码"：格式为 base64url(ed25519 签名) + "." + base64url(payload JSON)
func ParseUnlockCode(code string) (module string, nbf, exp time.Time, err error) {
	parts := strings.Split(code, ".")
	if len(parts) != 2 {
		return "", time.Time{}, time.Time{}, errors.New("授权码格式错误")
	}
	pub := PublicKey()
	if pub == nil {
		return "", time.Time{}, time.Time{}, errors.New("授权公钥未配置")
	}
	sig, sErr := base64.RawURLEncoding.DecodeString(parts[0])
	if sErr != nil {
		return "", time.Time{}, time.Time{}, errors.New("授权码签名解码失败")
	}
	payload, pErr := base64.RawURLEncoding.DecodeString(parts[1])
	if pErr != nil {
		return "", time.Time{}, time.Time{}, errors.New("授权码载荷解码失败")
	}
	if !ed25519.Verify(pub, payload, sig) {
		return "", time.Time{}, time.Time{}, errors.New("授权码签名校验失败（授权码无效或已被篡改）")
	}
	var pl struct {
		Module string `json:"module"`
		Nbf    int64  `json:"nbf"`
		Exp    int64  `json:"exp"`
	}
	if jErr := json.Unmarshal(payload, &pl); jErr != nil {
		return "", time.Time{}, time.Time{}, errors.New("授权码载荷解析失败")
	}
	return pl.Module, time.Unix(pl.Nbf, 0), time.Unix(pl.Exp, 0), nil
}

// VerifyUnlockCode 校验授权码在当前时间有效（nbf<=now<=exp）。
func VerifyUnlockCode(code, module string, now time.Time) (bool, error) {
	m, nbf, exp, err := ParseUnlockCode(code)
	if err != nil {
		return false, err
	}
	if m != "" && m != module {
		return false, errors.New("授权码模块不匹配")
	}
	if now.Before(nbf) {
		return false, errors.New("授权码尚未生效（未到生效时间）")
	}
	if now.After(exp) {
		return false, errors.New("授权码已过期")
	}
	return true, nil
}

// FormatTrialDate 输出“有效期至”的友好时间
func FormatTrialDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// FormatDurationDays 剩余天数（负数表示已过期）
func FormatDurationDays(d time.Duration) int {
	return int(d.Hours() / 24)
}

var _ = strconv.Itoa
