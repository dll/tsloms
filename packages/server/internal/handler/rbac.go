package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// RBAC 管理接口
// 权限点/角色/用户权限 的查询与维护。全部仅管理员可访问（路由层已用
// middleware.RequirePerm("role:manage") 拦截）。
// ============================================================================

// MyPermissions 当前登录用户的角色与有效功能权限（供前端菜单/按钮联动）
func MyPermissions(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("user_role")
	perms, err := model.EffectivePermissionCodes(userID)
	if err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"role": role, "permissions": perms})
}

// ListPermissions 权限点字典（按模块分组排序返回）
func ListPermissions(c *gin.Context) {
	var perms []model.Permission
	model.DB.Order("sort ASC, id ASC").Find(&perms)

	// 按模块分组
	modules := []gin.H{}
	byModule := map[string][]gin.H{}
	order := []string{}
	seen := map[string]bool{}
	for _, p := range perms {
		if _, ok := byModule[p.Module]; !ok {
			byModule[p.Module] = []gin.H{}
			order = append(order, p.Module)
		}
		byModule[p.Module] = append(byModule[p.Module], gin.H{
			"id": p.ID, "code": p.Code, "name": p.Name, "module": p.Module, "sort": p.Sort,
		})
		seen[p.Code] = true
	}
	_ = seen
	for _, m := range order {
		modules = append(modules, gin.H{"module": m, "permissions": byModule[m]})
	}
	ok(c, gin.H{"list": modules, "total": len(perms)})
}

// ListRoles 角色列表（含每个角色的默认权限编码）
func ListRoles(c *gin.Context) {
	var roles []model.Role
	model.DB.Order("builtin DESC, id ASC").Find(&roles)

	// 角色 → 权限编码
	permCodeByID := map[uint]string{}
	var perms []model.Permission
	model.DB.Find(&perms)
	for _, p := range perms {
		permCodeByID[p.ID] = p.Code
	}

	list := make([]gin.H, 0, len(roles))
	for _, r := range roles {
		var rps []model.RolePermission
		model.DB.Where("role_id = ?", r.ID).Find(&rps)
		codes := []string{}
		for _, rp := range rps {
			if c, ok := permCodeByID[rp.PermissionID]; ok {
				codes = append(codes, c)
			}
		}
		list = append(list, gin.H{
			"id": r.ID, "code": r.Code, "name": r.Name, "builtin": r.Builtin,
			"description": r.Description, "permissions": codes, "created_at": r.CreatedAt,
		})
	}
	ok(c, gin.H{"list": list, "total": len(list)})
}

// CreateRole 创建自定义角色
func CreateRole(c *gin.Context) {
	var req struct {
		Code        string   `json:"code" binding:"required"`
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（code/name 必填）")
		return
	}
	if req.Code == model.BuiltinRoleAdmin || req.Code == model.BuiltinRoleOperator || req.Code == model.BuiltinRoleViewer {
		badRequest(c, "不能创建与内置角色同名的角色")
		return
	}
	var count int64
	model.DB.Model(&model.Role{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		badRequest(c, "角色编码已存在")
		return
	}

	role := model.Role{Code: req.Code, Name: req.Name, Description: req.Description, Builtin: false}
	if err := model.DB.Create(&role).Error; err != nil {
		serverError(c, err)
		return
	}
	// 写权限关联
	setRolePermissions(role.ID, role.Code, req.Permissions)
	recordOperation(c, model.OpCreate, "role/"+role.Code, "创建角色")
	ok(c, gin.H{"id": role.ID, "code": role.Code, "message": "角色创建成功"})
}

// UpdateRole 更新自定义角色（名称/描述/权限）
func UpdateRole(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "角色ID无效")
		return
	}
	var role model.Role
	if err := model.DB.First(&role, id).Error; err != nil {
		notFound(c, "角色不存在")
		return
	}
	if role.Builtin {
		badRequest(c, "内置角色不可编辑，可通过用户级权限覆写调整")
		return
	}
	var req struct {
		Name        *string   `json:"name"`
		Description *string   `json:"description"`
		Permissions *[]string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	if req.Name != nil {
		model.DB.Model(&role).Update("name", *req.Name)
	}
	if req.Description != nil {
		model.DB.Model(&role).Update("description", *req.Description)
	}
	if req.Permissions != nil {
		setRolePermissions(role.ID, role.Code, *req.Permissions)
	}
	recordOperation(c, model.OpUpdate, "role/"+role.Code, "更新角色")
	ok(c, gin.H{"message": "角色更新成功"})
}

// DeleteRole 删除自定义角色（内置角色与有用户的角色不可删）
func DeleteRole(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "角色ID无效")
		return
	}
	var role model.Role
	if err := model.DB.First(&role, id).Error; err != nil {
		notFound(c, "角色不存在")
		return
	}
	if role.Builtin {
		badRequest(c, "内置角色不可删除")
		return
	}
	var userCount int64
	model.DB.Model(&model.User{}).Where("role = ?", role.Code).Count(&userCount)
	if userCount > 0 {
		badRequest(c, "该角色已有用户绑定，不可删除")
		return
	}
	model.DB.Where("role_id = ?", role.ID).Delete(&model.RolePermission{})
	model.DB.Delete(&role)
	recordOperation(c, model.OpDelete, "role/"+role.Code, "删除角色")
	ok(c, gin.H{"message": "角色删除成功"})
}

// setRolePermissions 覆盖角色权限关联
func setRolePermissions(roleID uint, roleCode string, codes []string) {
	model.DB.Where("role_id = ?", roleID).Delete(&model.RolePermission{})
	for _, code := range codes {
		var p model.Permission
		if err := model.DB.Where("code = ?", code).First(&p).Error; err != nil {
			continue
		}
		model.DB.Create(&model.RolePermission{RoleID: roleID, PermissionID: p.ID, RoleCode: roleCode})
	}
}

// GetUserPermissions 某用户的有效权限编码（供编辑用户时回显）
func GetUserPermissions(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "用户ID无效")
		return
	}
	var user model.User
	if err := model.DB.First(&user, id).Error; err != nil {
		notFound(c, "用户不存在")
		return
	}
	// 角色默认权限
	var perms []model.RolePermission
	var role model.Role
	roleDefaults := []string{}
	if err := model.DB.Where("code = ?", user.Role).First(&role).Error; err == nil {
		model.DB.Where("role_id = ?", role.ID).Find(&perms)
	}
	permCodeByID := map[uint]string{}
	var allPerms []model.Permission
	model.DB.Find(&allPerms)
	for _, p := range allPerms {
		permCodeByID[p.ID] = p.Code
	}
	for _, rp := range perms {
		if c, ok := permCodeByID[rp.PermissionID]; ok {
			roleDefaults = append(roleDefaults, c)
		}
	}

	// 用户级覆写
	var ups []model.UserPermission
	model.DB.Where("user_id = ?", id).Find(&ups)
	userGrants := []string{}
	userDenies := []string{}
	for _, up := range ups {
		if up.Granted {
			userGrants = append(userGrants, up.Permission)
		} else {
			userDenies = append(userDenies, up.Permission)
		}
	}

	ok(c, gin.H{
		"role":          user.Role,
		"role_defaults": roleDefaults,
		"user_grants":   userGrants,
		"user_denies":   userDenies,
	})
}

// SetUserPermissions 设置某用户的用户级权限覆写
// grants: 显式授权项（若为空列表且无 denies，则继承角色默认）
// denies: 显式拒绝项（从角色默认中剔除，仅当无 grants 时生效）
func SetUserPermissions(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "用户ID无效")
		return
	}
	var user model.User
	if err := model.DB.First(&user, id).Error; err != nil {
		notFound(c, "用户不存在")
		return
	}
	// 内置 admin 恒为全权限，禁止覆写
	if user.Role == model.BuiltinRoleAdmin && user.Username == "admin" {
		badRequest(c, "内置 admin 拥有全部权限，不可覆写")
		return
	}
	var req struct {
		Grants []string `json:"grants"`
		Denies []string `json:"denies"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	// 校验权限编码合法
	valid := map[string]bool{}
	for _, p := range model.AllPermissions {
		valid[p.Code] = true
	}
	model.DB.Where("user_id = ?", id).Delete(&model.UserPermission{})
	if len(req.Grants) > 0 {
		for _, code := range req.Grants {
			if !valid[code] {
				continue
			}
			model.DB.Create(&model.UserPermission{UserID: id, Permission: code, Granted: true})
		}
	} else {
		// 无显式授权：写入拒绝项（剔除默认）
		for _, code := range req.Denies {
			if !valid[code] {
				continue
			}
			model.DB.Create(&model.UserPermission{UserID: id, Permission: code, Granted: false})
		}
	}
	recordOperation(c, model.OpUpdate, "user/"+user.Username+"/permissions", "设置用户功能权限")
	ok(c, gin.H{"message": "功能权限已更新"})
}
