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

// ListUsers 用户列表查询（分页 + 角色/部门/状态筛选）
// 仅管理员可访问
func ListUsers(c *gin.Context) {
	page, pageSize := paginate(c)

	query := model.DB.Model(&model.User{})
	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if dept := c.Query("department_id"); dept != "" {
		if did, err := parseUint(dept); err == nil && did > 0 {
			query = query.Where("department_id = ?", did)
		}
	}
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("username LIKE ? OR real_name LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	query.Count(&total)

	var users []model.User
	query.Order("id ASC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&users)

	// 部门名称映射
	deptNames := map[uint]string{}
	var deptIDs []uint
	for _, u := range users {
		if u.DepartmentID != nil {
			deptIDs = append(deptIDs, *u.DepartmentID)
		}
	}
	if len(deptIDs) > 0 {
		var depts []model.Department
		model.DB.Where("id IN ?", deptIDs).Find(&depts)
		for _, d := range depts {
			deptNames[d.ID] = d.Name
		}
	}

	// 不返回密码哈希
	safeUsers := make([]gin.H, 0, len(users))
	for _, u := range users {
		deptName := ""
		if u.DepartmentID != nil {
			deptName = deptNames[*u.DepartmentID]
		}
		safeUsers = append(safeUsers, gin.H{
			"id": u.ID, "username": u.Username, "role": u.Role,
			"real_name":     u.RealName,
			"phone":         u.Phone,
			"email":         u.Email,
			"department_id": u.DepartmentID,
			"department":    deptName,
			"status":        u.Status,
			"last_login_at": u.LastLoginAt,
			"created_at":    u.CreatedAt,
		})
	}

	ok(c, gin.H{"list": safeUsers, "total": total, "page": page, "page_size": pageSize})
}

// validatePasswordStrength 强密码校验：至少10位，且同时包含字母与数字（审计 #2 建议）
func validatePasswordStrength(pw string) string {
	if len(pw) < 10 {
		return "密码至少10位"
	}
	hasLetter := false
	hasDigit := false
	for _, r := range pw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return "密码需同时包含字母和数字"
	}
	return ""
}

// CreateUser 创建用户（管理员）
func CreateUser(c *gin.Context) {
	var req struct {
		Username      string `json:"username" binding:"required"`
		Password      string `json:"password" binding:"required"`
		Role          string `json:"role" binding:"required"`
		RealName      string `json:"real_name"`
		Phone         string `json:"phone"`
		Email         string `json:"email"`
		DepartmentID  *uint  `json:"department_id"`
		WorkNo        string `json:"work_no"`
		Avatar        string `json:"avatar"`
		Gender        string `json:"gender"`
		IDCard        string `json:"id_card"`
		Address       string `json:"address"`
		Education     string `json:"education"`
		EngineerLevel string `json:"engineer_level"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（用户名/密码必填）")
		return
	}
	if msg := validatePasswordStrength(req.Password); msg != "" {
		badRequest(c, msg)
		return
	}
	if !validRole(req.Role) {
		badRequest(c, "无效的角色")
		return
	}
	// 手机号：若提供则校验格式（11 位大陆手机号）；不强制必填（兼容旧账号/测试）
	phone := strings.TrimSpace(req.Phone)
	if phone != "" && !validPhoneFormat(phone) {
		badRequest(c, "手机号格式不正确（需 11 位大陆手机号）")
		return
	}

	// 检查用户名唯一
	var count int64
	model.DB.Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		badRequest(c, "用户名已存在")
		return
	}
	// 校验部门存在
	if req.DepartmentID != nil {
		var dept model.Department
		if err := model.DB.First(&dept, *req.DepartmentID).Error; err != nil {
			badRequest(c, "部门不存在")
			return
		}
	}

	user := model.User{
		Username:      strings.TrimSpace(req.Username),
		PasswordHash:  model.HashPassword(req.Password),
		Role:          req.Role,
		RealName:      strings.TrimSpace(req.RealName),
		Phone:         phone,
		Email:         strings.TrimSpace(req.Email),
		DepartmentID:  req.DepartmentID,
		Status:        model.UserStatusEnabled,
		WorkNo:        strings.TrimSpace(req.WorkNo),
		Avatar:        strings.TrimSpace(req.Avatar),
		Gender:        strings.TrimSpace(req.Gender),
		IDCard:        strings.TrimSpace(req.IDCard),
		Address:       strings.TrimSpace(req.Address),
		Education:     strings.TrimSpace(req.Education),
		EngineerLevel: strings.TrimSpace(req.EngineerLevel),
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
		Role          *string `json:"role"`
		RealName      *string `json:"real_name"`
		Phone         *string `json:"phone"`
		Email         *string `json:"email"`
		DepartmentID  *uint   `json:"department_id"`
		Status        *string `json:"status"`
		WorkNo        *string `json:"work_no"`
		Avatar        *string `json:"avatar"`
		Gender        *string `json:"gender"`
		IDCard        *string `json:"id_card"`
		Address       *string `json:"address"`
		Education     *string `json:"education"`
		EngineerLevel *string `json:"engineer_level"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if req.Role != nil && validRole(*req.Role) {
		updates["role"] = *req.Role
	}
	if req.RealName != nil {
		updates["real_name"] = strings.TrimSpace(*req.RealName)
	}
	if req.Phone != nil {
		p := strings.TrimSpace(*req.Phone)
		if p != "" && !validPhoneFormat(p) {
			badRequest(c, "手机号格式不正确（需 11 位大陆手机号）")
			return
		}
		updates["phone"] = p
		if p != "" {
			updates["phone_login"] = p // 手机号即登录账号，同步更新
		}
	}
	if req.Email != nil {
		updates["email"] = strings.TrimSpace(*req.Email)
	}
	if req.WorkNo != nil {
		updates["work_no"] = strings.TrimSpace(*req.WorkNo)
	}
	if req.Avatar != nil {
		updates["avatar"] = strings.TrimSpace(*req.Avatar)
	}
	if req.Gender != nil {
		updates["gender"] = strings.TrimSpace(*req.Gender)
	}
	if req.IDCard != nil {
		updates["id_card"] = strings.TrimSpace(*req.IDCard)
	}
	if req.Address != nil {
		updates["address"] = strings.TrimSpace(*req.Address)
	}
	if req.Education != nil {
		updates["education"] = strings.TrimSpace(*req.Education)
	}
	if req.EngineerLevel != nil {
		updates["engineer_level"] = strings.TrimSpace(*req.EngineerLevel)
	}
	if req.DepartmentID != nil {
		var dept model.Department
		if err := model.DB.First(&dept, *req.DepartmentID).Error; err != nil {
			badRequest(c, "部门不存在")
			return
		}
		updates["department_id"] = *req.DepartmentID
	}
	if req.Status != nil {
		if *req.Status != model.UserStatusEnabled && *req.Status != model.UserStatusDisabled {
			badRequest(c, "无效的用户状态")
			return
		}
		updates["status"] = *req.Status
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
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "密码必填")
		return
	}
	if msg := validatePasswordStrength(req.Password); msg != "" {
		badRequest(c, msg)
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

// validPhoneFormat 校验大陆手机号格式（11 位，1 开头）
func validPhoneFormat(phone string) bool {
	if len(phone) != 11 {
		return false
	}
	if phone[0] != '1' {
		return false
	}
	for i := 0; i < len(phone); i++ {
		if phone[i] < '0' || phone[i] > '9' {
			return false
		}
	}
	return true
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

// UpdateMyCenter 设置/清除当前用户的地图中心（该用户管辖区域中心点）
func UpdateMyCenter(c *gin.Context) {
	var req struct {
		Lat *float64 `json:"lat"`
		Lng *float64 `json:"lng"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	userID := c.GetUint("user_id")
	if userID == 0 {
		unauthorized(c, "未登录")
		return
	}
	updates := map[string]interface{}{"center_lat": req.Lat, "center_lng": req.Lng}
	if req.Lat != nil && (*req.Lat < -90 || *req.Lat > 90) {
		badRequest(c, "纬度范围 -90~90")
		return
	}
	if req.Lng != nil && (*req.Lng < -180 || *req.Lng > 180) {
		badRequest(c, "经度范围 -180~180")
		return
	}
	if err := model.DB.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, "user/self/center", "设置个人地图中心点")
	ok(c, gin.H{"message": "地图中心点已更新"})
}
