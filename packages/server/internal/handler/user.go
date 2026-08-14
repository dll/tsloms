package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListAssignableUsers 可派单人员列表（operator + admin）
// 供工单派单下拉选择，登录用户（运维/管理员/查看）均可调用
func ListAssignableUsers(c *gin.Context) {
	var users []model.User
	model.DB.Select("id, username, role").
		Where("role IN ?", []string{model.RoleAdmin, model.RoleOperator}).
		Order("id ASC").
		Find(&users)
	list := make([]gin.H, 0, len(users))
	for _, u := range users {
		list = append(list, gin.H{"id": u.ID, "username": u.Username, "role": u.Role})
	}
	ok(c, gin.H{"list": list, "total": len(list)})
}

// ListUsers 用户列表查询（分页 + 角色筛选）
// 仅管理员可访问
func ListUsers(c *gin.Context) {
	page, _ := parseUint(c.DefaultQuery("page", "1"))
	pageSize, _ := parseUint(c.DefaultQuery("page_size", "20"))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := model.DB.Model(&model.User{})
	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("username LIKE ?", "%"+keyword+"%")
	}

	var total int64
	query.Count(&total)

	var users []model.User
	query.Order("id ASC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&users)

	// 不返回密码哈希
	safeUsers := make([]gin.H, 0, len(users))
	for _, u := range users {
		safeUsers = append(safeUsers, gin.H{
			"id": u.ID, "username": u.Username, "role": u.Role,
			"phone": u.Phone, "created_at": u.CreatedAt,
		})
	}

	ok(c, gin.H{"list": safeUsers, "total": total, "page": page, "page_size": pageSize})
}

// CreateUser 创建用户（管理员）
func CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Role     string `json:"role" binding:"required"`
		Phone    string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（用户名/密码必填，密码至少6位）")
		return
	}
	if !validRole(req.Role) {
		badRequest(c, "无效的角色")
		return
	}

	// 检查用户名唯一
	var count int64
	model.DB.Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		badRequest(c, "用户名已存在")
		return
	}

	user := model.User{
		Username:     strings.TrimSpace(req.Username),
		PasswordHash: model.HashPassword(req.Password),
		Role:         req.Role,
		Phone:        req.Phone,
	}
	if err := model.DB.Create(&user).Error; err != nil {
		serverError(c, err)
		return
	}

	recordOperation(c, model.OpCreate, "user/"+req.Username, "创建用户")
	ok(c, gin.H{"id": user.ID, "username": user.Username, "role": user.Role, "message": "用户创建成功"})
}

// UpdateUser 更新用户（角色/手机号，管理员）
func UpdateUser(c *gin.Context) {
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

	var req struct {
		Role  *string `json:"role"`
		Phone *string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if req.Role != nil && validRole(*req.Role) {
		updates["role"] = *req.Role
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if len(updates) > 0 {
		if err := model.DB.Model(&user).Updates(updates).Error; err != nil {
			serverError(c, err)
			return
		}
	}

	recordOperation(c, model.OpUpdate, "user/"+user.Username, "更新用户信息")
	ok(c, gin.H{"id": user.ID, "username": user.Username, "role": user.Role, "phone": user.Phone, "message": "用户更新成功"})
}

// ResetUserPassword 重置用户密码（管理员）
func ResetUserPassword(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "用户ID无效")
		return
	}
	var req struct {
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "密码必填且至少6位")
		return
	}

	var user model.User
	if err := model.DB.First(&user, id).Error; err != nil {
		notFound(c, "用户不存在")
		return
	}

	if err := model.DB.Model(&user).Update("password_hash", model.HashPassword(req.Password)).Error; err != nil {
		serverError(c, err)
		return
	}

	recordOperation(c, model.OpUpdate, "user/"+user.Username+"/password", "重置用户密码")
	ok(c, gin.H{"message": "密码重置成功"})
}

// DeleteUser 删除用户（管理员，禁止删除自己）
func DeleteUser(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "用户ID无效")
		return
	}
	// 禁止删除当前登录用户
	if c.GetUint("user_id") == id {
		badRequest(c, "不能删除当前登录用户")
		return
	}

	var user model.User
	if err := model.DB.First(&user, id).Error; err != nil {
		notFound(c, "用户不存在")
		return
	}
	if user.Username == "admin" {
		badRequest(c, "内置 admin 账号不可删除")
		return
	}
	if err := model.DB.Delete(&user).Error; err != nil {
		serverError(c, err)
		return
	}

	recordOperation(c, model.OpDelete, "user/"+user.Username, "删除用户")
	ok(c, gin.H{"message": "用户删除成功"})
}

// validRole 校验角色合法性
func validRole(role string) bool {
	return role == model.RoleAdmin || role == model.RoleOperator || role == model.RoleViewer
}

// UpdateMyPhone 修改当前登录用户手机号
func UpdateMyPhone(c *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "手机号不能为空")
		return
	}
	userID := c.GetUint("user_id")
	if userID == 0 {
		unauthorized(c, "未登录")
		return
	}
	if err := model.DB.Model(&model.User{}).Where("id = ?", userID).Update("phone", req.Phone).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, "user/self/phone", "修改个人手机号")
	ok(c, gin.H{"message": "手机号已更新"})
}
