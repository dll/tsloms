package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
)

func corsEngine(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	hw := CORS()
	if cfg != nil {
		hw = CORS()
		_ = cfg
	}
	r.Use(hw)
	r.GET("/api/v1/ping", func(c *gin.Context) { c.String(200, "pong") })
	return r
}

func TestCORS_DevEchoesOrigin(t *testing.T) {
	config.ResetCache()
	// 默认 AppEnv=development → 开发模式：回显 Origin
	r := corsEngine(nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/ping", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	r.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("dev 应回显 origin, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_DevWildcard(t *testing.T) {
	config.ResetCache()
	r := corsEngine(nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ping", nil)) // 无 Origin
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("dev 无 origin 应通配, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_Preflight(t *testing.T) {
	config.ResetCache()
	r := corsEngine(nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/api/v1/ping", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	r.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Errorf("preflight 应 204, got %d", w.Code)
	}
}

func TestCORS_ProductionWhitelist(t *testing.T) {
	config.ResetCache()
	cfg := config.Get() // Get() 填充缓存，CORS() 读同一实例
	cfg.AppEnv = "production"
	cfg.AllowedOrigins = "https://admin.tsloms.cn, http://127.0.0.1:8092"
	// 切换生产模式（CORS() 构造时内部读 config.Get() = cfg）
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/api/v1/ping", func(c *gin.Context) { c.String(200, "pong") })

	// 白名单内 Origin 放行
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/api/v1/ping", nil)
	req1.Header.Set("Origin", "https://admin.tsloms.cn")
	r.ServeHTTP(w1, req1)
	if w1.Header().Get("Access-Control-Allow-Origin") != "https://admin.tsloms.cn" {
		t.Errorf("白名单 Origin 应放行, got %q", w1.Header().Get("Access-Control-Allow-Origin"))
	}

	// 非白名单 Origin 不放行
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/api/v1/ping", nil)
	req2.Header.Set("Origin", "https://evil.com")
	r.ServeHTTP(w2, req2)
	if w2.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("非白名单 Origin 不应放行, got %q", w2.Header().Get("Access-Control-Allow-Origin"))
	}
	config.ResetCache() // 还原避免影响其他测试
}

func TestLogger_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Logger())
	r.GET("/api/v1/ping", func(c *gin.Context) { c.String(200, "pong") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ping", nil))
	if w.Code != http.StatusOK {
		t.Errorf("logger 包内请求应 200, got %d", w.Code)
	}
}
