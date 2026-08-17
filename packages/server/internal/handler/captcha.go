package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 算术验证码（登录防暴力，替代短信/图形验证码）
// ----------------------------------------------------------------------------
// 参考项目 a（RuoYi）用 GET /code 返回图形验证码图片 {uuid,img}，登录带 code+uuid。
// 本实现改为**简单算术题**（如 "2 + 8 = ?"，答 10 通过），更轻量：
//   - GET /auth/captcha  → { uuid, question, question_text }（如 "2 + 8 = ?"）
//   - 登录请求带 { username(可手机号), password, captcha_uuid, captcha_code }
//   - 后端校验 captcha_code 是否等于该 uuid 对应算式结果
//
// 存储：进程内内存 map（本系统单实例部署，够用；带过期与校验次数防御暴力试算）。
// ============================================================================

// captchaEntry 一条算术验证码记录
type captchaEntry struct {
	Answer    int       // 算式结果
	ExpiresAt time.Time // 过期时间
	VerifyCnt int       // 已校验次数（防暴力）
	Question  string    // 展示题目，如 "2 + 8 = ?"
}

// captchaStore 内存验证码存储
var captchaStore = struct {
	sync.RWMutex
	m map[string]*captchaEntry
}{m: make(map[string]*captchaEntry)}

// captchaTTL 验证码有效期
const captchaTTL = 5 * time.Minute

// captchaMaxVerify 最大校验次数
const captchaMaxVerify = 20

// randomInt 生成 [0,max) 随机整数
func randomInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

// generateCaptcha 生成一条算术题，返回 uuid 展示题目与正确答案。
func generateCaptcha() (uuid, question string, answer int) {
	// 简单加减：a + b 或 a - b（结果保证非负），范围 0-9 便于口算
	a := randomInt(10)
	b := randomInt(10)
	if randomInt(2) == 0 {
		// 加法
		answer = a + b
		question = fmt.Sprintf("%d + %d = ?", a, b)
	} else {
		// 减法（保证 a >= b 结果非负）
		if a < b {
			a, b = b, a
		}
		answer = a - b
		question = fmt.Sprintf("%d - %d = ?", a, b)
	}

	// 生成 8 字节随机 uuid
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	uuid = hex.EncodeToString(buf)

	captchaStore.Lock()
	captchaStore.m[uuid] = &captchaEntry{
		Answer:    answer,
		ExpiresAt: time.Now().Add(captchaTTL),
		Question:  question,
	}
	captchaStore.Unlock()
	return uuid, question, answer
}

// GetCaptcha GET /auth/captcha
// 返回 { uuid, question }（如 "2 + 8 = ?"）；前端展示题目，用户输入答案提交登录。
// 公开接口（无需登录）。
func GetCaptcha(c *gin.Context) {
	uuid, question, _ := generateCaptcha()
	ok(c, gin.H{
		"uuid":     uuid,
		"question": question,
	})
}

// verifyCaptcha 校验算术验证码（一次性；带次数防御与过期）。
// 返回 true 表示通过（并同时标记该 uuid 作废）。
func verifyCaptcha(uuid, code string) bool {
	if uuid == "" || code == "" {
		return false
	}
	// 解析答案：支持前置负号（减法可能为 0-9）
	var ans int
	neg := false
	s := code
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
		ans = ans*10 + int(s[i]-'0')
	}
	if neg {
		ans = -ans
	}

	captchaStore.Lock()
	defer captchaStore.Unlock()

	entry, ok := captchaStore.m[uuid]
	if !ok {
		return false // 不存在或已作废
	}
	// 过期
	if time.Now().After(entry.ExpiresAt) {
		delete(captchaStore.m, uuid)
		return false
	}
	// 次数防御
	entry.VerifyCnt++
	if entry.VerifyCnt > captchaMaxVerify {
		delete(captchaStore.m, uuid)
		return false
	}
	// 比对答案
	if entry.Answer != ans {
		return false
	}
	// 通过：作废（一次性）
	delete(captchaStore.m, uuid)
	return true
}
