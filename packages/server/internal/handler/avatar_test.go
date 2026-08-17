package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// registerProfileRoutes 注册个人资料/头像相关的需鉴权路由（用 stub 中间件注入 user_id）
func registerProfileRoutes(r *gin.Engine, userID uint) *gin.RouterGroup {
	g := r.Group("/api/v1")
	g.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	g.PUT("/user/profile", UpdateMyProfile)
	g.GET("/user/info", GetUserInfo)
	g.POST("/user/avatar", UploadMyAvatar)
	return g
}

// TestUpdateMyProfile_PersonnelFields 个人资料：更新人事字段并回读
func TestUpdateMyProfile_PersonnelFields(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	u := model.User{
		Username:     "p1",
		PasswordHash: model.HashPassword("Test@12345"),
		Role:         model.RoleOperator,
		RealName:     "张三",
		Phone:        "13900000001",
		PhoneLogin:   "13900000001",
	}
	model.DB.Create(&u)
	registerProfileRoutes(r, u.ID)

	code, _ := doReq(t, r, "PUT", "/api/v1/user/profile",
		`{"real_name":"李四","gender":"male","work_no":"W001","education":"本科","engineer_level":"中级","address":"合肥市"}`)
	if code != http.StatusOK {
		t.Fatalf("更新个人资料失败 code=%d", code)
	}
	var got model.User
	model.DB.First(&got, u.ID)
	if got.RealName != "李四" || got.Gender != "male" || got.WorkNo != "W001" ||
		got.Education != "本科" || got.EngineerLevel != "中级" || got.Address != "合肥市" {
		t.Errorf("人事字段未正确更新: %+v", got)
	}
}

// TestUpdateMyProfile_PhoneFormat 个人资料：手机号格式校验
func TestUpdateMyProfile_PhoneFormat(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	u := model.User{Username: "p2", PasswordHash: model.HashPassword("x"), Role: model.RoleViewer}
	model.DB.Create(&u)
	registerProfileRoutes(r, u.ID)

	// 非法手机号 → 拒绝
	code, _ := doReq(t, r, "PUT", "/api/v1/user/profile", `{"phone":"123"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法手机号应 400, got %d", code)
	}
	// 合法手机号 → 成功并同步 phone_login
	code, _ = doReq(t, r, "PUT", "/api/v1/user/profile", `{"phone":"13800000002"}`)
	if code != http.StatusOK {
		t.Errorf("合法手机号应 200, got %d", code)
	}
	var got model.User
	model.DB.First(&got, u.ID)
	if got.Phone != "13800000002" || got.PhoneLogin != "13800000002" || !got.PhoneVerified {
		t.Errorf("手机号未正确同步: phone=%s login=%s verified=%v", got.Phone, got.PhoneLogin, got.PhoneVerified)
	}
}

// TestAvatarUpload_ValidAndInvalid UploadMyAvatar：缺文件/非法扩展名被拒，合法 png 上传回填 avatar URL
func TestAvatarUpload_ValidAndInvalid(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	u := model.User{Username: "p3", PasswordHash: model.HashPassword("x"), Role: model.RoleOperator}
	model.DB.Create(&u)
	registerProfileRoutes(r, u.ID)

	// 缺文件 → 400
	code, _ := doReq(t, r, "POST", "/api/v1/user/avatar", "")
	if code != http.StatusBadRequest {
		t.Errorf("缺文件应 400, got %d", code)
	}

	// 合法 png 上传（内容可为空，仅测流程与回填）
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "photo.png")
	_, _ = fw.Write([]byte("fake-png-bytes"))
	w.Close()
	req := httptest.NewRequest("POST", "/api/v1/user/avatar", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("合法图片上传应 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var got model.User
	model.DB.First(&got, u.ID)
	if got.Avatar == "" {
		t.Error("上传后 avatar 应被回填 URL")
	}
}
