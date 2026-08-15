package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
	"golang.org/x/crypto/bcrypt"
)

// newUserEngine 构造用户管理路由（需注入管理员上下文）
func newUserEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	// 模拟管理员上下文
	setAdmin := func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_role", model.RoleAdmin)
		c.Set("username", "admin")
		c.Next()
	}
	users := r.Group("/users")
	users.Use(setAdmin)
	{
		users.GET("", ListUsers)
		users.POST("", CreateUser)
		users.PUT("/:id", UpdateUser)
		users.PUT("/:id/password", ResetUserPassword)
		users.DELETE("/:id", DeleteUser)
	}
	return r
}

func TestCreateUser_AndList(t *testing.T) {
	r := newUserEngine(t)
	// 创建用户
	body := `{"username":"operator1","password":"StrongPass123","role":"operator","phone":"13800000000"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/users", bodyReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("创建用户状态码=%d body=%s", w.Code, w.Body.String())
	}

	// 列表应包含 1 个用户
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/users", nil))
	var resp struct {
		Code float64 `json:"code"`
		Data struct {
			Total float64      `json:"total"`
			List  []model.User `json:"list"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp.Data.Total != 1 {
		t.Errorf("用户总数 = %v, 期望 1", resp.Data.Total)
	}
	// 列表不应返回 password_hash
	if len(resp.Data.List) > 0 && resp.Data.List[0].PasswordHash != "" {
		t.Error("列表不应返回密码哈希")
	}
}

func TestCreateUser_Duplicate(t *testing.T) {
	r := newUserEngine(t)
	post := `{"username":"u1","password":"StrongPass123","role":"viewer"}`
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/users", bodyReader(post)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/users", bodyReader(post)))
	if w.Code != http.StatusBadRequest {
		t.Errorf("重复用户名状态码 = %d, 期望 400", w.Code)
	}
}

func TestCreateUser_WeakPassword(t *testing.T) {
	r := newUserEngine(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/users", bodyReader(`{"username":"x","password":"123","role":"viewer"}`)))
	if w.Code != http.StatusBadRequest {
		t.Errorf("弱密码状态码 = %d, 期望 400", w.Code)
	}
}

func TestDeleteUser_NotSelf(t *testing.T) {
	r := newUserEngine(t)
	// 先建一个用户 id=1
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/users", bodyReader(`{"username":"a","password":"StrongPass123","role":"viewer"}`)))
	// 当前模拟 user_id=1，删除自己应被拒
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/users/1", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("删除自己被拒状态码 = %d, 期望 400", w.Code)
	}
}

func TestResetUserPassword(t *testing.T) {
	r := newUserEngine(t)
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/users", bodyReader(`{"username":"b","password":"StrongPass123","role":"viewer"}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/users/1/password", bodyReader(`{"password":"NewPass2024"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("重置密码状态码 = %d body=%s", w.Code, w.Body.String())
	}
	// 验证新密码可登录（调用 Login handler）
	// 用 model 直接验证哈希
	var u model.User
	model.DB.First(&u, 1)
	if err := verifyPassword(u.PasswordHash, "NewPass2024"); err != nil {
		t.Errorf("新密码校验失败: %v", err)
	}
	if err := verifyPassword(u.PasswordHash, "StrongPass123"); err == nil {
		t.Error("旧密码不应再有效")
	}
}

// bodyReader 构造 JSON 请求体
func bodyReader(s string) *bytes.Reader {
	return bytes.NewReader([]byte(s))
}

// verifyPassword 校验 bcrypt 哈希
func verifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
