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

// TestAuth_DisabledUser 停用用户的既有有效令牌应立即失效（P1-03）
func TestAuth_DisabledUser(t *testing.T) {
	model.InitTestDB()
	u := model.User{Username: "disabled_user", PasswordHash: model.HashPassword("x"), Role: model.RoleViewer, Status: model.UserStatusDisabled}
	if err := model.DB.Create(&u).Error; err != nil {
		t.Fatalf("创建停用用户失败: %v", err)
	}
	cfg := config.Load()
	cfg.JWTSecret = "test-secret"
	// 令牌本身有效（未过期、签名正确）
	tok := signToken(t, u.ID, u.Role, cfg.JWTSecret, time.Now().Add(time.Hour))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(cfg))
	r.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"uid": c.GetUint("user_id")}) })

	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("停用用户有效令牌状态码 = %d, 期望 401; body=%s", w.Code, w.Body.String())
	}
}

// TestAuth_EnabledUserStillAccessible 启用用户正常访问（防止误伤）
func TestAuth_EnabledUserStillAccessible(t *testing.T) {
	model.InitTestDB()
	u := model.User{Username: "enabled_user", PasswordHash: model.HashPassword("x"), Role: model.RoleViewer, Status: model.UserStatusEnabled}
	if err := model.DB.Create(&u).Error; err != nil {
		t.Fatalf("创建启用用户失败: %v", err)
	}
	cfg := config.Load()
	cfg.JWTSecret = "test-secret"
	tok := signToken(t, u.ID, u.Role, cfg.JWTSecret, time.Now().Add(time.Hour))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(cfg))
	r.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"uid": c.GetUint("user_id")}) })

	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("启用用户状态码 = %d, 期望 200; body=%s", w.Code, w.Body.String())
	}
}

// TestRequirePerm 功能权限中间件（ai:ops 等）越权拒绝
func TestRequirePerms_OR(t *testing.T) {
	model.InitTestDB()
	gin.SetMode(gin.TestMode)
	// 用户具备 ai:ops（覆盖 viewer）
	u := model.User{Username: "perms_or", PasswordHash: "x", Role: model.RoleViewer, Status: model.UserStatusEnabled}
	model.DB.Create(&u)
	model.DB.Create(&model.UserPermission{UserID: u.ID, Permission: "ai:ops", Granted: true})

	handler := func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) }
	setUID := func(uid uint) gin.HandlerFunc {
		return func(c *gin.Context) { c.Set("user_id", uid); c.Next() }
	}

	r := gin.New()
	// OR: ai:ops 或 device:read 任一即可 → 200
	r.GET("/or", setUID(u.ID), RequirePerms("device:read", "ai:ops"), handler)
	// 无 user_id → 401
	r.GET("/or-nouid", RequirePerms("ai:ops"), handler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/or", nil))
	if w.Code != http.StatusOK {
		t.Errorf("OR 任一权限应 200, got %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/or-nouid", nil))
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("OR 无 user_id 应 401, got %d", w2.Code)
	}
}

func TestRequirePerms_None(t *testing.T) {
	model.InitTestDB()
	gin.SetMode(gin.TestMode)
	// viewer 无任何额外权限
	u := model.User{Username: "perms_none", PasswordHash: "x", Role: model.RoleViewer, Status: model.UserStatusEnabled}
	model.DB.Create(&u)
	setUID := func(uid uint) gin.HandlerFunc {
		return func(c *gin.Context) { c.Set("user_id", uid); c.Next() }
	}
	handler := func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) }
	r := gin.New()
	r.GET("/none", setUID(u.ID), RequirePerms("ai:ops", "device:create"), handler)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/none", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("无任一权限应 403, got %d", w.Code)
	}
}

func TestRequirePerm(t *testing.T) {
	model.InitTestDB()
	gin.SetMode(gin.TestMode)

	// 创建两个用户：一个有 ai:ops，一个没有
	withPerm := model.User{Username: "perm_on", PasswordHash: model.HashPassword("x"), Role: model.RoleViewer, Status: model.UserStatusEnabled}
	if err := model.DB.Create(&withPerm).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	// 用户级授权 ai:ops（覆盖 viewer 默认，使其具备该权限）
	if err := model.DB.Create(&model.UserPermission{UserID: withPerm.ID, Permission: "ai:ops", Granted: true}).Error; err != nil {
		t.Fatalf("授予权限失败: %v", err)
	}
	noPerm := model.User{Username: "perm_off", PasswordHash: model.HashPassword("x"), Role: model.RoleViewer, Status: model.UserStatusEnabled}
	if err := model.DB.Create(&noPerm).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	handler := func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) }
	setUID := func(uid uint) gin.HandlerFunc {
		return func(c *gin.Context) { c.Set("user_id", uid); c.Next() }
	}

	r := gin.New()
	r.GET("/has-ai", setUID(withPerm.ID), RequirePerm("ai:ops"), handler)
	r.GET("/no-ai", setUID(noPerm.ID), RequirePerm("ai:ops"), handler)
	r.GET("/no-uid", RequirePerm("ai:ops"), handler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/has-ai", nil))
	if w.Code != http.StatusOK {
		t.Errorf("具备 ai:ops 调用 = %d, 期望 200; body=%s", w.Code, w.Body.String())
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/no-ai", nil))
	if w2.Code != http.StatusForbidden {
		t.Errorf("无 ai:ops 调用 = %d, 期望 403; body=%s", w2.Code, w2.Body.String())
	}

	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest("GET", "/no-uid", nil))
	if w3.Code != http.StatusUnauthorized {
		t.Errorf("无 user_id 调用 = %d, 期望 401", w3.Code)
	}
}
