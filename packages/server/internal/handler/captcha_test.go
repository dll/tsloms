package handler

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestCaptcha_Get_ReturnsUUIDAndQuestion 算术验证码接口：返回 uuid + 算式题目，公开无需登录
func TestCaptcha_Get_ReturnsUUIDAndQuestion(t *testing.T) {
	r := gin.New()
	g := r.Group("/api/v1")
	g.GET("/auth/captcha", GetCaptcha)
	code, body := doReq(t, r, "GET", "/api/v1/auth/captcha", "")
	if code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("获取验证码失败 code=%d body=%v", code, body)
	}
	data := body["data"].(map[string]interface{})
	uuid, ok1 := data["uuid"].(string)
	question, ok2 := data["question"].(string)
	if !ok1 || uuid == "" {
		t.Errorf("验证码应返回非空 uuid, got %q", uuid)
	}
	if !ok2 || question == "" {
		t.Errorf("验证码应返回题目, got %q", question)
	}
}

// TestCaptcha_Verify_Wrong_Then_Right 验证：错误答案被拒，正确答案通过（且一次性）
func TestCaptcha_Verify_Wrong_Then_Right(t *testing.T) {
	uuid, _, ans := generateCaptcha()
	if verifyCaptcha(uuid, "999999999") {
		t.Error("错误答案应被拒绝")
	}
	if !verifyCaptcha(uuid, strconv.Itoa(ans)) {
		t.Error("正确答案应通过")
	}
	// 一次性：再次用过期的 uuid 应失败
	if verifyCaptcha(uuid, strconv.Itoa(ans)) {
		t.Error("验证码应一次性，重复使用应失败")
	}
}
