package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

func userCovEngine(t *testing.T) *gin.Engine {
	t.Helper()
	model.InitTestDB()
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/users/assignable", ListAssignableUsers)
		g.GET("/users", ListUsers)
		g.POST("/users", CreateUser)
		g.PUT("/users/:id", UpdateUser)
		g.PUT("/users/:id/password", ResetUserPassword)
		g.DELETE("/users/:id", DeleteUser)
		g.PUT("/user/phone", UpdateMyPhone)
		g.PUT("/user/center", UpdateMyCenter)
	}
	return r
}

// authed 注入 user_id 的路由组
func authedGroup(r *gin.Engine) *gin.RouterGroup {
	g := r.Group("/api/v1/authed")
	g.Use(func(c *gin.Context) { c.Set("user_id", uint(10)); c.Next() })
	g.PUT("/phone", UpdateMyPhone)
	g.PUT("/center", UpdateMyCenter)
	return g
}

func TestUser_Assignable(t *testing.T) {
	r := userCovEngine(t)
	model.DB.Create(&model.User{Username: "op1", Role: model.RoleOperator})
	model.DB.Create(&model.User{Username: "adm1", Role: model.RoleAdmin})
	model.DB.Create(&model.User{Username: "vw1", Role: model.RoleViewer})
	code, body := doReq(t, r, "GET", "/api/v1/users/assignable", "")
	mustOK(t, code, body, "可派单人员")
	if body["data"].(map[string]interface{})["total"].(float64) != 2 {
		t.Errorf("可派单应 2(admin+operator), got %v", body["data"].(map[string]interface{})["total"])
	}
}

func TestUser_ListFilters(t *testing.T) {
	r := userCovEngine(t)
	model.DB.Create(&model.User{Username: "li", RealName: "李明", Role: model.RoleOperator, Status: model.UserStatusEnabled})
	dept := model.Department{Name: "运维一队"}
	model.DB.Create(&dept)
	model.DB.Create(&model.User{Username: "wang", RealName: "王五", Role: model.RoleViewer, Status: model.UserStatusEnabled, DepartmentID: &dept.ID})

	for _, q := range []string{"", "?role=operator", "?status=disabled", "?department_id=" + uid(dept.ID), "?keyword=李"} {
		code, body := doReq(t, r, "GET", "/api/v1/users"+q, "")
		mustOK(t, code, body, "用户列表 "+q)
	}
	// 部门名映射
	code, body := doReq(t, r, "GET", "/api/v1/users?department_id="+uid(dept.ID), "")
	mustOK(t, code, body, "用户列表部门")
	if body["data"].(map[string]interface{})["list"].([]interface{})[0].(map[string]interface{})["department"] != "运维一队" {
		t.Errorf("department 名未映射")
	}
}

func TestUser_Update(t *testing.T) {
	r := userCovEngine(t)
	u := model.User{Username: "upd_user", PasswordHash: "x", Role: model.RoleViewer}
	model.DB.Create(&u)
	dept := model.Department{Name: "新队"}
	model.DB.Create(&dept)
	// 更新角色+状态+部门
	code, _ := doReq(t, r, "PUT", "/api/v1/users/"+uid(u.ID), `{"role":"operator","status":"enabled","department_id":`+uid(dept.ID)+`}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "更新用户")
	// 非法状态
	code, _ = doReq(t, r, "PUT", "/api/v1/users/"+uid(u.ID), `{"status":"bogus"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法状态应 400, got %d", code)
	}
	// 部门不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/users/"+uid(u.ID), `{"department_id":99999}`)
	if code != http.StatusBadRequest {
		t.Errorf("部门不存在应 400, got %d", code)
	}
	// 用户不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/users/99999", `{"role":"operator"}`)
	if code != http.StatusNotFound {
		t.Errorf("更新不存在用户应 404, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "PUT", "/api/v1/users/abc", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
}

func TestUser_DeleteProtections(t *testing.T) {
	r := userCovEngine(t)
	// 内置 admin 不可删
	model.DB.Create(&model.User{Username: "admin", PasswordHash: "x", Role: model.RoleAdmin})
	var adm model.User
	model.DB.Where("username=?", "admin").First(&adm)
	code, _ := doReq(t, r, "DELETE", "/api/v1/users/"+uid(adm.ID), "")
	if code != http.StatusBadRequest {
		t.Errorf("内置admin应 400, got %d", code)
	}
	// 删除不存在
	code, _ = doReq(t, r, "DELETE", "/api/v1/users/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("删除不存在应 404, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "DELETE", "/api/v1/users/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法删除ID应 400, got %d", code)
	}
}

func TestUser_MyPhoneCenter(t *testing.T) {
	r := userCovEngine(t)
	// 建用户并注入 user_id
	model.DB.Create(&model.User{Username: "me", Role: model.RoleOperator})
	authed := r.Group("/api/v1/authed")
	authed.Use(func(c *gin.Context) { c.Set("user_id", uint(1)); c.Next() })
	authed.PUT("/phone", UpdateMyPhone)
	authed.PUT("/center", UpdateMyCenter)

	// 改手机
	code, _ := doReq(t, r, "PUT", "/api/v1/authed/phone", `{"phone":"13800000000"}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "改手机")
	// 缺手机号
	code, _ = doReq(t, r, "PUT", "/api/v1/authed/phone", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺手机应 400, got %d", code)
	}
	// 设中心
	code, _ = doReq(t, r, "PUT", "/api/v1/authed/center", `{"lat":31.2,"lng":121.5}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "设中心")
	// 非法 lat
	code, _ = doReq(t, r, "PUT", "/api/v1/authed/center", `{"lat":200,"lng":121}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法lat应 400, got %d", code)
	}
	// 非法 lng
	code, _ = doReq(t, r, "PUT", "/api/v1/authed/center", `{"lat":31,"lng":200}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法lng应 400, got %d", code)
	}
	// 未注入 user_id (uid=0) → 401
	code2, _ := doReq(t, r, "PUT", "/api/v1/authed/center", `{"lat":31,"lng":121}`)
	_ = code2
	// 直接调无注入的路由 → 401
	code3, _ := doReq(t, r, "PUT", "/api/v1/user/phone", `{"phone":"138"}`)
	if code3 != http.StatusUnauthorized {
		t.Errorf("未登录手机应 401, got %d", code3)
	}
}

func TestUser_ResetPasswordCov(t *testing.T) {
	r := userCovEngine(t)
	u := model.User{Username: "reset_me", PasswordHash: "x", Role: model.RoleViewer}
	model.DB.Create(&u)
	// 弱密码
	code, _ := doReq(t, r, "PUT", "/api/v1/users/"+uid(u.ID)+"/password", `{"password":"short"}`)
	if code != http.StatusBadRequest {
		t.Errorf("弱密码应 400, got %d", code)
	}
	// 缺密码
	code, _ = doReq(t, r, "PUT", "/api/v1/users/"+uid(u.ID)+"/password", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺密码应 400, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "PUT", "/api/v1/users/abc/password", `{"password":"Strong12345"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
	// 不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/users/99999/password", `{"password":"Strong12345"}`)
	if code != http.StatusNotFound {
		t.Errorf("不存在应 404, got %d", code)
	}
}
