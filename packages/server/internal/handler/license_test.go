package handler

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// licenseRoutes 注册授权路由（用注入 user_id/role 的 stub 中间件，模拟超管）
func licenseRoutes(r *gin.Engine) {
	g := r.Group("/api/v1")
	g.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Set("user_role", model.RoleSuperAdmin); c.Next() })
	g.POST("/license/trial/start", StartTrial)
	g.GET("/license/status", GetLicenseStatus)
	g.POST("/license/unlock", UnlockLicense)
}

// helper: 测试用供应方私钥（与服务器公钥配对）签发授权码
const licenseTestPrivB64 = "QNn9Qbk-Pi2AYWhbeHl2_SAYfLBbTLYfFNYsXxCpoSH8iWnoFYNHnzBjfD2uyduZj1DL89E9TaW0-wek4D36Cw"

func signLicenseCode(t *testing.T, module string, nbf, exp time.Time) string {
	t.Helper()
	privBytes, err := base64.RawURLEncoding.DecodeString(licenseTestPrivB64)
	if err != nil || len(privBytes) != ed25519.PrivateKeySize {
		t.Fatal("私钥解码失败")
	}
	priv := ed25519.PrivateKey(privBytes)
	raw, _ := json.Marshal(map[string]any{"module": module, "nbf": nbf.Unix(), "exp": exp.Unix()})
	sig := ed25519.Sign(priv, raw)
	return base64.RawURLEncoding.EncodeToString(sig) + "." + base64.RawURLEncoding.EncodeToString(raw)
}

func TestLicense_StartTrialCoreAndStatus(t *testing.T) {
	r := gin.New()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	licenseRoutes(r)

	// 开始核心试用
	code, _ := doReq(t, r, "POST", "/api/v1/license/trial/start", `{"module":"core"}`)
	if code != http.StatusOK {
		t.Fatalf("开始核心试用失败 code=%d", code)
	}
	// 状态应含 core trial
	_, body := doReq(t, r, "GET", "/api/v1/license/status", "")
	list := body["data"].(map[string]interface{})["list"].([]interface{})
	core := list[0].(map[string]interface{})
	if core["key"] != "core" || core["state"] != "trial" {
		t.Errorf("核心状态应为 trial, got %v", core)
	}
}

func TestLicense_StartTrialOptionalAndExpiry(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	licenseRoutes(r)
	_, _ = doReq(t, r, "POST", "/api/v1/license/trial/start", `{"module":"ai"}`)
	_, body := doReq(t, r, "GET", "/api/v1/license/status", "")
	list := body["data"].(map[string]interface{})["list"].([]interface{})
	for _, it := range list {
		item := it.(map[string]interface{})
		if item["key"] == "ai" {
			if item["state"] != "trial" {
				t.Errorf("ai 模块应 trial, got %v", item["state"])
			}
			return
		}
	}
	t.Fatal("未找到 ai 模块状态")
}

func TestLicense_UnlockBySuperAdmin(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	licenseRoutes(r)
	_, _ = doReq(t, r, "POST", "/api/v1/license/trial/start", `{"module":"ai"}`)
	// 超管一键解锁（code 空）
	code, _ := doReq(t, r, "POST", "/api/v1/license/unlock", `{"module":"ai"}`)
	if code != http.StatusOK {
		t.Fatalf("一键解锁失败 code=%d", code)
	}
	_, body := doReq(t, r, "GET", "/api/v1/license/status", "")
	for _, it := range body["data"].(map[string]interface{})["list"].([]interface{}) {
		item := it.(map[string]interface{})
		if item["key"] == "ai" && item["state"] != "unlocked" {
			t.Errorf("ai 应 unlocked, got %v", item["state"])
		}
	}
}

func TestLicense_UnlockByCode(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	licenseRoutes(r)
	now := time.Now()
	code := signLicenseCode(t, "ai", now.Add(-time.Hour), now.Add(365*24*time.Hour))
	c, _ := doReq(t, r, "POST", "/api/v1/license/unlock", `{"module":"ai","code":"`+code+`"}`)
	if c != http.StatusOK {
		t.Fatalf("授权码解锁失败 code=%d", c)
	}
}

func TestLicense_RejectWrongCode(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	licenseRoutes(r)
	c, _ := doReq(t, r, "POST", "/api/v1/license/unlock", `{"module":"ai","code":"INVALID.CODE"}`)
	if c != http.StatusUnauthorized {
		t.Errorf("无效授权码应 401, got %d", c)
	}
}

func TestLicense_UnknownModuleRejected(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	licenseRoutes(r)
	c, _ := doReq(t, r, "POST", "/api/v1/license/trial/start", `{"module":"nope"}`)
	if c != http.StatusBadRequest {
		t.Errorf("未知模块应 400, got %d", c)
	}
}
