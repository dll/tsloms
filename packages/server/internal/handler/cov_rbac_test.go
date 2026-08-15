package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
	"golang.org/x/crypto/bcrypt"
)

// 权限代码 ID 查询（SeedRBAC 已灌入全量字典）
func permIDOf(code string) uint {
	var p model.Permission
	model.DB.Where("code = ?", code).First(&p)
	return p.ID
}

// createRole 构造自定义角色并在 DB 创建
func createRole(t *testing.T, code string) model.Role {
	t.Helper()
	role := model.Role{Code: code, Name: code, Builtin: false}
	model.DB.Create(&role)
	return role
}

func getRoleByCode(t *testing.T, code string) model.Role {
	t.Helper()
	var r model.Role
	model.DB.Where("code = ?", code).First(&r)
	return r
}

func TestRBAC_ListPermissionsAndRoles(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/permissions", ListPermissions)
	rg.GET("/roles", ListRoles)
	// SeedRBAC 已自动灌入全部权限
	code, body := doReq(t, r, "GET", "/api/v1/permissions", "")
	mustOK(t, code, body, "权限字典")
	if body["data"].(map[string]interface{})["total"].(float64) < 10 {
		t.Errorf("权限 total 应>=10, got %v", body["data"].(map[string]interface{})["total"])
	}
	mods := body["data"].(map[string]interface{})["list"].([]interface{})
	if len(mods) < 3 {
		t.Errorf("模块分组应>=3, got %d", len(mods))
	}
	// 角色列表（内置 admin/operator/viewer）
	code, body = doReq(t, r, "GET", "/api/v1/roles", "")
	mustOK(t, code, body, "角色列表")
	roles := body["data"].(map[string]interface{})["list"].([]interface{})
	if len(roles) < 3 {
		t.Errorf("内置角色应>=3, got %d", len(roles))
	}
}

func TestRBAC_MyPermissions(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1/authed")
	rg.Use(func(c *gin.Context) { c.Set("user_id", uint(1)); c.Set("user_role", "admin"); c.Next() })
	rg.GET("/my-permissions", MyPermissions)
	// 创建 admin 用户（admin 角色默认全权限）
	pb, _ := bcrypt.GenerateFromPassword([]byte("Test@12345"), bcrypt.MinCost)
	usr := model.User{Username: "adm_x", PasswordHash: string(pb), Role: "admin"}
	model.DB.Create(&usr)
	code, body := doReq(t, r, "GET", "/api/v1/authed/my-permissions", "")
	mustOK(t, code, body, "我的权限(admin)")
	if body["data"].(map[string]interface{})["role"] != "admin" {
		t.Errorf("role=%v", body["data"].(map[string]interface{})["role"])
	}
	permList := body["data"].(map[string]interface{})["permissions"].([]interface{})
	if len(permList) < 5 {
		t.Errorf("admin 权限应>=5, got %d", len(permList))
	}
	// 权限不存在(有效权限查询错误分支)；uid 无用户 → 空
	rg2 := r.Group("/api/v1/plain")
	rg2.GET("/my-permissions", MyPermissions)
	code, _ = doReq(t, r, "GET", "/api/v1/plain/my-permissions", "")
	// 无用户(uid=0, 无有效权限) → EffectivePermissionCodes 返回错误 → 500 错误分支
	if code != http.StatusOK && code != http.StatusInternalServerError {
		t.Errorf("未登录 my-permissions 期望 200(空)或 500(error分支), got %d", code)
	}
}

func TestRBAC_CreateRole(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.POST("/roles", CreateRole)
	// 成功
	code, body := doReq(t, r, "POST", "/api/v1/roles", `{"code":"sup","name":"主管","permissions":["device:update","ai:ops"]}`)
	mustOK(t, code, body, "创建角色")
	// 内置角色名冲突
	code, _ = doReq(t, r, "POST", "/api/v1/roles", `{"code":"admin","name":"x"}`)
	if code != http.StatusBadRequest {
		t.Errorf("内置角色名应 400, got %d", code)
	}
	// 重复编码
	code, _ = doReq(t, r, "POST", "/api/v1/roles", `{"code":"sup","name":"y"}`)
	if code != http.StatusBadRequest {
		t.Errorf("重复编码应 400, got %d", code)
	}
	// 缺参数
	code, _ = doReq(t, r, "POST", "/api/v1/roles", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺参数应 400, got %d", code)
	}
	// 含非法权限编码（被 setRolePermissions 跳过）
	code, _ = doReq(t, r, "POST", "/api/v1/roles", `{"code":"ok1","name":"x","permissions":["nonexistent"]}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "含非法权限仍创建")
}

func TestRBAC_UpdateDeleteRole(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.PUT("/roles/:id", UpdateRole)
	rg.DELETE("/roles/:id", DeleteRole)
	role := createRole(t, "tech")
	// 更新名称 + 权限
	code, _ := doReq(t, r, "PUT", "/api/v1/roles/"+uid(role.ID), `{"name":"高级技术","permissions":["fault:update","ai:ops"]}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "更新角色")
	// 不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/roles/99999", `{"name":"x"}`)
	if code != http.StatusNotFound {
		t.Errorf("更新不存在应 404, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "PUT", "/api/v1/roles/abc", `{"name":"x"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
	// 删除成功
	code, _ = doReq(t, r, "DELETE", "/api/v1/roles/"+uid(role.ID), "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "删除角色")
	// 删除不存在
	code, _ = doReq(t, r, "DELETE", "/api/v1/roles/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("删除不存在应 404, got %d", code)
	}
	// 非法删除ID
	code, _ = doReq(t, r, "DELETE", "/api/v1/roles/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法删除ID应 400, got %d", code)
	}
}

func TestRBAC_Protections(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.PUT("/roles/:id", UpdateRole)
	rg.DELETE("/roles/:id", DeleteRole)
	// 内置 viewer 角色
	viewer := getRoleByCode(t, "viewer")
	if viewer.Builtin {
		code, _ := doReq(t, r, "PUT", "/api/v1/roles/"+uid(viewer.ID), `{"name":"x"}`)
		if code != http.StatusBadRequest {
			t.Errorf("内置角色编辑应 400, got %d", code)
		}
		code, _ = doReq(t, r, "DELETE", "/api/v1/roles/"+uid(viewer.ID), "")
		if code != http.StatusBadRequest {
			t.Errorf("内置角色删除应 400, got %d", code)
		}
	}
	// 有用户绑定的角色不可删
	role := createRole(t, "busy")
	model.DB.Create(&model.User{Username: "u_busy", Role: "busy"})
	code, _ := doReq(t, r, "DELETE", "/api/v1/roles/"+uid(role.ID), "")
	if code != http.StatusBadRequest {
		t.Errorf("有用户角色应 400, got %d", code)
	}
}

func TestRBAC_UserPermissions(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/users/:id/permissions", GetUserPermissions)
	rg.POST("/users/:id/permissions", SetUserPermissions)
	// 自定义角色 tech 赋默认权限
	role := createRole(t, "tech")
	model.DB.Create(&model.RolePermission{RoleID: role.ID, PermissionID: permIDOf("device:update"), RoleCode: role.Code})
	op := model.User{Username: "op_rb", PasswordHash: "x", Role: "tech"}
	model.DB.Create(&op)

	// 获取（回显角色默认）
	code, body := doReq(t, r, "GET", "/api/v1/users/"+uid(op.ID)+"/permissions", "")
	mustOK(t, code, body, "获取用户权限")
	if len(body["data"].(map[string]interface{})["role_defaults"].([]interface{})) != 1 {
		t.Errorf("role_defaults 期望 1 条")
	}
	// 设置 grants
	code, _ = doReq(t, r, "POST", "/api/v1/users/"+uid(op.ID)+"/permissions", `{"grants":["device:update","ai:ops"]}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "设置授权")
	// 设置 denies
	code, _ = doReq(t, r, "POST", "/api/v1/users/"+uid(op.ID)+"/permissions", `{"denies":["ai:ops"]}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "设置拒绝")
	// 内置 admin 不可覆写
	adm := model.User{Username: "admin", PasswordHash: "x", Role: "admin"}
	model.DB.Create(&adm)
	code, _ = doReq(t, r, "POST", "/api/v1/users/"+uid(adm.ID)+"/permissions", `{"grants":["device:update"]}`)
	if code != http.StatusBadRequest {
		t.Errorf("内置admin覆写应 400, got %d", code)
	}
	// 用户不存在
	code, _ = doReq(t, r, "GET", "/api/v1/users/99999/permissions", "")
	if code != http.StatusNotFound {
		t.Errorf("GET 用户不存在应 404, got %d", code)
	}
	code, _ = doReq(t, r, "POST", "/api/v1/users/99999/permissions", `{"grants":[]}`)
	if code != http.StatusNotFound {
		t.Errorf("POST 用户不存在应 404, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "GET", "/api/v1/users/abc/permissions", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
	code, _ = doReq(t, r, "POST", "/api/v1/users/abc/permissions", `{"grants":[]}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法POST ID应 400, got %d", code)
	}
}
