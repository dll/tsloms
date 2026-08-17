package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// P0-2 认证改造：双通道登录（手机号+验证码 / 用户名+密码）向后兼容
// ============================================================================

// p0AuthEngine 构造带公开认证路由的测试引擎
func p0AuthEngine(t *testing.T) *gin.Engine {
	t.Helper()
	config.ResetCache() // 重置配置缓存（读取测试环境变量）
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.POST("/auth/login", Login)
		api.POST("/auth/sms-code", SendSmsCode)
	}
	return r
}

// p0CreatePasswordUser 创建带密码的用户（内置角色 admin/operator/viewer）
func p0CreatePasswordUser(username, password, phone string) model.User {
	u := model.User{
		Username:      username,
		PasswordHash:  model.HashPassword(password),
		Role:          model.RoleOperator,
		Phone:         phone,
		PhoneLogin:    phone,
		PhoneVerified: true,
	}
	model.DB.Create(&u)
	return u
}

func TestPwdLogin_BackwardCompatible(t *testing.T) {
	r := p0AuthEngine(t)
	p0CreatePasswordUser("olduser", "secret123", "13800000001")

	// 旧语义：{username, password}
	code, body := doReq(t, r, "POST", "/api/v1/auth/login", `{"username":"olduser","password":"secret123"}`)
	if code != 200 || body["code"].(float64) != 0 {
		t.Fatalf("旧密码登录失败: code=%d body=%v", code, body)
	}
	data := body["data"].(map[string]interface{})
	if data["token"] == "" {
		t.Errorf("旧密码登录应返回 token")
	}
	user := data["user"].(map[string]interface{})
	if user["phone_login"] == nil {
		t.Errorf("user 应返回 phone_login 可选字段（旧账号兼容）")
	}
}

func TestPwdLogin_WrongPassword(t *testing.T) {
	r := p0AuthEngine(t)
	p0CreatePasswordUser("u1", "good", "13800000002")
	code, _ := doReq(t, r, "POST", "/api/v1/auth/login", `{"username":"u1","password":"bad"}`)
	if code != 401 {
		t.Errorf("错误密码应 401, 实际 %d", code)
	}
}

func TestSmsCode_SendAndVerify_ConsoleChannel(t *testing.T) {
	r := p0AuthEngine(t)
	// 配置为 console（默认），返回 debug.code 供开发联调
	p0CreatePasswordUser("smuser", "pw", "13900000001")

	// 1) 发送验证码
	code, body := doReq(t, r, "POST", "/api/v1/auth/sms-code", `{"phone":"13900000001"}`)
	if code != 200 || body["code"].(float64) != 0 {
		t.Fatalf("发送验证码失败: code=%d body=%v", code, body)
	}
	dbg := body["data"].(map[string]interface{})["debug"].(map[string]interface{})
	if dbg["dev"] != true {
		t.Errorf("console 通道应返回 dev 调试信息")
	}
	smsCode := dbg["code"].(string)
	if smsCode == "" {
		t.Fatalf("console 通道应回显验证码")
	}

	// 2) 手机号+验证码登录（login_type=sms）
	lcode, lbody := doReq(t, r, "POST", "/api/v1/auth/login",
		`{"phone":"13900000001","code":"`+smsCode+`","login_type":"sms"}`)
	if lcode != 200 || lbody["code"].(float64) != 0 {
		t.Fatalf("验证码登录失败: code=%d body=%v", lcode, lbody)
	}
}

func TestSmsCode_ReuseRejected(t *testing.T) {
	r := p0AuthEngine(t)
	p0CreatePasswordUser("smuser2", "pw", "13900000002")
	// 发送并取码
	_, body := doReq(t, r, "POST", "/api/v1/auth/sms-code", `{"phone":"13900000002"}`)
	smsCode := body["data"].(map[string]interface{})["debug"].(map[string]interface{})["code"].(string)

	// 第一次成功
	if lc, lb := doReq(t, r, "POST", "/api/v1/auth/login",
		`{"phone":"13900000002","code":"`+smsCode+`","login_type":"sms"}`); lc != 200 || lb["code"].(float64) != 0 {
		t.Fatalf("首次验证码登录应成功: %v", lb)
	}
	// 第二次复用同一验证码应失败（一次性）
	if lc, _ := doReq(t, r, "POST", "/api/v1/auth/login",
		`{"phone":"13900000002","code":"`+smsCode+`","login_type":"sms"}`); lc != 401 {
		t.Errorf("复用已用验证码应 401, 实际 %d", lc)
	}
}

func TestSmsCode_WrongCodeRejected(t *testing.T) {
	r := p0AuthEngine(t)
	p0CreatePasswordUser("smuser3", "pw", "13900000003")
	_, _ = doReq(t, r, "POST", "/api/v1/auth/sms-code", `{"phone":"13900000003"}`)
	lc, _ := doReq(t, r, "POST", "/api/v1/auth/login",
		`{"phone":"13900000003","code":"000000","login_type":"sms"}`)
	if lc != 401 {
		t.Errorf("错误验证码应 401, 实际 %d", lc)
	}
}

func TestSmsCode_InvalidPhone(t *testing.T) {
	r := p0AuthEngine(t)
	code, _ := doReq(t, r, "POST", "/api/v1/auth/sms-code", `{"phone":"123"}`)
	if code != 400 {
		t.Errorf("非法手机号应 400, 实际 %d", code)
	}
}
