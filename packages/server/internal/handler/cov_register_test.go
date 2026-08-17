package handler

import (
	"net/http"
	"regexp"
	"strconv"
	"testing"

	"github.com/tsloms/server/internal/model"
)

// parseMathAnswer 从算术题 "2 + 8 = ?" 解析答案
func parseMathAnswer(q string) string {
	re := regexp.MustCompile(`(-?\d+)\s*([+\-])\s*(-?\d+)`)
	m := re.FindStringSubmatch(q)
	if len(m) != 4 {
		return ""
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[3])
	if m[2] == "+" {
		return strconv.Itoa(a + b)
	}
	return strconv.Itoa(a - b)
}

// TestRegister 用户自助注册：参数/校验/建号/唯一性
func TestRegister(t *testing.T) {
	r := covSetup(t) // 初始化测试 DB（model.DB）
	api := r.Group("/api/v1")
	api.POST("/auth/register", Register)
	api.GET("/auth/captcha", GetCaptcha)

	// 缺用户名/密码 → 400
	code, _ := doReq(t, r, "POST", "/api/v1/auth/register", `{"password":"123456"}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺用户名应 400, got %d", code)
	}
	// 密码过短 → 400
	code, _ = doReq(t, r, "POST", "/api/v1/auth/register", `{"username":"u1","password":"123"}`)
	if code != http.StatusBadRequest {
		t.Errorf("密码过短应 400, got %d", code)
	}
	// 验证码错误 → 401
	code, _ = doReq(t, r, "POST", "/api/v1/auth/register", `{"username":"u1","password":"123456","captcha_uuid":"x","captcha_code":"999"}`)
	if code != http.StatusUnauthorized {
		t.Errorf("验证码错误应 401, got %d", code)
	}

	// 正常注册（先取验证码）
	cjCode, cjBody := doReq(t, r, "GET", "/api/v1/auth/captcha", "")
	if cjCode != http.StatusOK {
		t.Fatalf("captcha 应200, got %d", cjCode)
	}
	cd := cjBody["data"].(map[string]interface{})
	uuid := cd["uuid"].(string)
	// 题目形如 "2 + 8 = ?"，解析答案
	question := cd["question"].(string)
	ans := parseMathAnswer(question)
	if ans == "" {
		t.Fatalf("无法解析题目: %s", question)
	}

	code, body := doReq(t, r, "POST", "/api/v1/auth/register",
		`{"username":"reg_u1","password":"123456","real_name":"张三","phone":"13800000001","captcha_uuid":"`+uuid+`","captcha_code":"`+ans+`"}`)
	if code != http.StatusOK {
		t.Fatalf("注册应 200, got %d: %v", code, body)
	}
	// 注册后应能登录（findUserByLogin）
	var u model.User
	if err := model.DB.Where("username = ?", "reg_u1").First(&u).Error; err != nil {
		t.Fatalf("注册用户应入库: %v", err)
	}
	if u.Role != model.RoleViewer || u.RealName != "张三" {
		t.Errorf("默认角色应为 viewer 且姓名正确: %+v", u)
	}

	// 重复用户名 → 400（重新取验证码）
	cjCode, cjBody = doReq(t, r, "GET", "/api/v1/auth/captcha", "")
	uuid2 := cjBody["data"].(map[string]interface{})["uuid"].(string)
	code, _ = doReq(t, r, "POST", "/api/v1/auth/register",
		`{"username":"reg_u1","password":"123456","captcha_uuid":"`+uuid2+`","captcha_code":"`+parseMathAnswer(cjBody["data"].(map[string]interface{})["question"].(string))+`"}`)
	if code != http.StatusBadRequest {
		t.Errorf("重复用户名应 400, got %d", code)
	}

	// 手机号已注册 → 400
	cjCode, cjBody = doReq(t, r, "GET", "/api/v1/auth/captcha", "")
	code, _ = doReq(t, r, "POST", "/api/v1/auth/register",
		`{"username":"reg_u2","password":"123456","phone":"13800000001","captcha_uuid":"`+cjBody["data"].(map[string]interface{})["uuid"].(string)+`","captcha_code":"`+parseMathAnswer(cjBody["data"].(map[string]interface{})["question"].(string))+`"}`)
	if code != http.StatusBadRequest {
		t.Errorf("重复手机号应 400, got %d", code)
	}

	// 清理
	model.DB.Where("username IN ?", []string{"reg_u1", "reg_u2"}).Delete(&model.User{})
}
