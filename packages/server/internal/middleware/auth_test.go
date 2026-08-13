package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

func newTestUser(t *testing.T) model.User {
	t.Helper()
	model.InitTestDB()
	u := model.User{Username: "tester", PasswordHash: model.HashPassword("x"), Role: model.RoleAdmin}
	if err := model.DB.Create(&u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return u
}

func signToken(t *testing.T, userID uint, role, secret string, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": float64(userID),
		"role":    role,
		"exp":     exp.Unix(),
	})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return s
}

func TestAuth_ValidToken(t *testing.T) {
	u := newTestUser(t)
	cfg := config.Load()
	cfg.JWTSecret = "test-secret"
	tok := signToken(t, u.ID, u.Role, cfg.JWTSecret, time.Now().Add(time.Hour))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(cfg))
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"uid": c.GetUint("user_id"), "role": c.GetString("user_role")})
	})

	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200; body=%s", w.Code, w.Body.String())
	}
}

func TestAuth_NoToken(t *testing.T) {
	newTestUser(t)
	cfg := config.Load()
	cfg.JWTSecret = "test-secret"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(cfg))
	r.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{}) })

	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("无 token 状态码 = %d, 期望 401", w.Code)
	}
}

func TestAuth_BadSignature(t *testing.T) {
	u := newTestUser(t)
	cfg := config.Load()
	cfg.JWTSecret = "test-secret"
	tok := signToken(t, u.ID, u.Role, "wrong-secret", time.Now().Add(time.Hour))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(cfg))
	r.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{}) })
	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("错误签名状态码 = %d, 期望 401", w.Code)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	u := newTestUser(t)
	cfg := config.Load()
	cfg.JWTSecret = "test-secret"
	tok := signToken(t, u.ID, u.Role, cfg.JWTSecret, time.Now().Add(-time.Hour))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(cfg))
	r.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{}) })
	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("过期 token 状态码 = %d, 期望 401", w.Code)
	}
}

func TestRequireOperator_RoleCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 先用一个中间件注入角色，再套 RequireOperator
	setRole := func(role string) gin.HandlerFunc {
		return func(c *gin.Context) { c.Set("user_role", role); c.Next() }
	}
	handler := func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) }

	// admin 通过
	r.GET("/op-admin", setRole(model.RoleAdmin), RequireOperator(), handler)
	// viewer 拒绝
	r.GET("/op-viewer", setRole(model.RoleViewer), RequireOperator(), handler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/op-admin", nil))
	if w.Code != http.StatusOK {
		t.Errorf("admin 调用 operator 接口 = %d, 期望 200", w.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/op-viewer", nil))
	if w2.Code != http.StatusForbidden {
		t.Errorf("viewer 调用 operator 接口 = %d, 期望 403", w2.Code)
	}
}
