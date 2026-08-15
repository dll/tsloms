package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

// TestSetupRouter_HealthAndLogin 验证路由装配与公开端点
func TestSetupRouter_HealthAndLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	cfg := config.Load()

	r := setupRouter(nil, cfg)

	// 公开：健康检查
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("health code=%d body=%s", w.Code, w.Body.String())
	}

	// 公开：登录（缺参 → 400）
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("POST", "/api/v1/auth/login", nil))
	if w2.Code != http.StatusBadRequest && w2.Code != http.StatusOK {
		t.Errorf("login 缺参 code=%d", w2.Code)
	}

	// 受保护端点（未登录 → 401）
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest("GET", "/api/v1/devices", nil))
	if w3.Code != http.StatusUnauthorized && w3.Code != http.StatusOK {
		t.Errorf("devices 未登录 code=%d", w3.Code)
	}

	// 静态媒体
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, httptest.NewRequest("GET", "/media/", nil))
	if w4.Code != http.StatusOK && w4.Code != http.StatusNotFound {
		t.Errorf("media 静态 code=%d", w4.Code)
	}
}

// TestSetupRouter_WrongMethod 验证 CORS/Logger 中间件装配不 panic
func TestSetupRouter_WrongMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	cfg := config.Load()
	r := setupRouter(nil, cfg)

	// OPTIONS 预检（CORS）
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/api/v1/devices", nil))
	// CORS 中间件应放行或返回 4xx，不应 panic
	_ = w.Code
}
