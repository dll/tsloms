package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// newDeptEngine 构造部门路由（管理员上下文 + 用户管理路由以验证部门关联）
func newDeptEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	setAdmin := func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_role", model.RoleAdmin)
		c.Set("username", "admin")
		c.Next()
	}
	depts := r.Group("/departments")
	depts.Use(setAdmin)
	{
		depts.GET("", ListDepartments)
		depts.POST("", CreateDepartment)
		depts.PUT("/:id", UpdateDepartment)
		depts.DELETE("/:id", DeleteDepartment)
	}
	users := r.Group("/users")
	users.Use(setAdmin)
	{
		users.POST("", CreateUser)
		users.GET("", ListUsers)
	}
	return r
}

func TestDepartment_CRUD(t *testing.T) {
	r := newDeptEngine(t)
	// 新增部门
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/departments", bodyReader(`{"name":"运维一部","leader":"张三"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("新增部门状态码=%d body=%s", w.Code, w.Body.String())
	}
	// 重复名称应拒绝
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("POST", "/departments", bodyReader(`{"name":"运维一部"}`)))
	if w2.Code != http.StatusBadRequest {
		t.Errorf("重复部门名状态码=%d 期望400", w2.Code)
	}
	// 更新部门
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest("PUT", "/departments/1", bodyReader(`{"description":"负责城市道路","leader":"李四"}`)))
	if w3.Code != http.StatusOK {
		t.Errorf("更新部门状态码=%d body=%s", w3.Code, w3.Body.String())
	}
	// 列表应包含部门
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, httptest.NewRequest("GET", "/departments", nil))
	var resp struct {
		Data struct {
			Total int `json:"total"`
			List  []struct {
				Name        string `json:"name"`
				MemberCount int64  `json:"member_count"`
			} `json:"list"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w4.Body.Bytes(), &resp)
	if resp.Data.Total != 1 || len(resp.Data.List) != 1 {
		t.Errorf("部门列表 total=%d len=%d 期望1", resp.Data.Total, len(resp.Data.List))
	}
	// 删除空部门
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, httptest.NewRequest("DELETE", "/departments/1", nil))
	if w5.Code != http.StatusOK {
		t.Errorf("删除部门状态码=%d body=%s", w5.Code, w5.Body.String())
	}
}

func TestDepartment_DeleteWithMembers_Rejected(t *testing.T) {
	r := newDeptEngine(t)
	// 建部门
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/departments", bodyReader(`{"name":"技术部"}`)))
	// 建用户并关联部门 1
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/users", bodyReader(`{"username":"op1","password":"StrongPass123","role":"operator","department_id":1}`)))
	// 删除该部门应被拒（仍有成员）
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/departments/1", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("带成员部门删除应拒绝, 状态码=%d body=%s", w.Code, w.Body.String())
	}
}

func TestUser_CreateWithDepartmentAndFilter(t *testing.T) {
	r := newDeptEngine(t)
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/departments", bodyReader(`{"name":"运维二部"}`)))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/users",
		bodyReader(`{"username":"op2","password":"StrongPass123","role":"operator","real_name":"王五","phone":"13900000000","department_id":1}`)))
	// 按部门筛选
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/users?department_id=1", nil))
	var resp struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Total != 1 {
		t.Errorf("按部门筛选用户 total=%d 期望1", resp.Data.Total)
	}
	// 用户名关键词匹配 real_name
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/users?keyword=王五", nil))
	_ = json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp.Data.Total != 1 {
		t.Errorf("按姓名关键词筛选 total=%d 期望1", resp.Data.Total)
	}
}
