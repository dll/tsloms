package handler

import (
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
	"golang.org/x/crypto/bcrypt"
)

// phoneRegex 手机号格式（11 位，1 开头大陆手机号）
var phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)

// LoginRequest 登录请求
// 登录方式：username(可手机号) + password + 算术验证码(captcha_uuid/captcha_code)
// 旧账号（username/password）不受影响，但需额外答对算术题（防暴力）。
type LoginRequest struct {
	// 登录账号（用户名或手机号）
	Username string `json:"username"`
	Password string `json:"password"`
	// 算术验证码：GET /auth/captcha 获取 uuid 与题目，此处提交答案
	CaptchaUUID string `json:"captcha_uuid"`
	CaptchaCode string `json:"captcha_code"`
}

// Login 登录：算术验证码 + 手机号作为登录账号（或用户名）。
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数错误")
		return
	}

	if req.Username == "" || req.Password == "" {
		badRequest(c, "请输入账号(用户名或手机号)和密码")
		return
	}

	// 算术验证码校验（防暴力破解；参考项目 a 用图形验证码，本实现用更轻量的算术题）
	if !verifyCaptcha(req.CaptchaUUID, req.CaptchaCode) {
		unauthorized(c, "算术验证码错误或已过期")
		return
	}

	// 查找用户（支持手机号作为登录账号或用户名）
	user := findUserByLogin(req.Username)
	if user == nil {
		unauthorized(c, "用户名或密码错误")
		return
	}

	// 验证密码（bcrypt）
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		unauthorized(c, "用户名或密码错误")
		return
	}

	// 检查账号状态
	if user.Status != "" && user.Status == model.UserStatusDisabled {
		unauthorized(c, "账号已停用，请联系管理员")
		return
	}

	// 更新最后登录时间
	now := time.Now()
	model.DB.Model(user).Update("last_login_at", now)

	// 签发 JWT
	cfg := config.Get()
	token, err := issueToken(user, cfg.JWTSecret)
	if err != nil {
		serverError(c, err)
		return
	}

	ok(c, gin.H{
		"token":           token,
		"user":            userPayload(user),
		"enabled_modules": EnabledModuleList(),
	})

	// 记录登录操作日志
	c.Set("op_username", user.Username)
	recordOperation(c, model.OpLogin, "auth/login", "用户登录")
}

// RegisterRequest 用户自助注册请求
// 参考项目 a 的用户注册（字段：deptId 归属部门 / username / password / confirmPassword / code 验证码），
// 并考虑本项目人事字段：real_name(姓名) / phone(手机号) / department_id(归属部门)。角色默认 viewer（只读）。
type RegisterRequest struct {
	Username     string `json:"username"`      // 登录用户名（或手机号）
	Password     string `json:"password"`      // 密码
	RealName     string `json:"real_name"`     // 姓名（与项目 a nickName 对应）
	Phone        string `json:"phone"`         // 手机号（可选，格式校验）
	DepartmentID *uint  `json:"department_id"` // 归属部门（对应项目 a 的 deptId，选填）
	CaptchaUUID  string `json:"captcha_uuid"`
	CaptchaCode  string `json:"captcha_code"`
}

// Register 用户自助注册（对外开放）：新建 viewer 角色账号（只读，最低权限）。
// 走算术验证码防滥用；用户名与手机号唯一性应用层校验。
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数错误")
		return
	}
	uname := strings.TrimSpace(req.Username)
	realName := strings.TrimSpace(req.RealName)
	if uname == "" || req.Password == "" {
		badRequest(c, "请输入用户名和密码")
		return
	}
	if len(req.Password) < 6 {
		badRequest(c, "密码长度至少 6 位")
		return
	}
	if realName == "" {
		realName = uname
	}
	// 算术验证码校验（防批量注册）
	if !verifyCaptcha(req.CaptchaUUID, req.CaptchaCode) {
		unauthorized(c, "算术验证码错误或已过期")
		return
	}

	// 归属部门（若填）需存在
	var deptID *uint
	if req.DepartmentID != nil && *req.DepartmentID > 0 {
		var dept model.Department
		if err := model.DB.First(&dept, *req.DepartmentID).Error; err != nil {
			badRequest(c, "归属部门不存在")
			return
		}
		deptID = req.DepartmentID
	}

	// 手机号格式（若填）
	phone := strings.TrimSpace(req.Phone)
	if phone != "" && !phoneRegex.MatchString(phone) {
		badRequest(c, "手机号格式不正确")
		return
	}

	// 用户名唯一
	var cnt int64
	model.DB.Model(&model.User{}).Where("username = ?", uname).Count(&cnt)
	if cnt > 0 {
		badRequest(c, "用户名已存在")
		return
	}
	// 手机号登录账号唯一（非空时）
	if phone != "" {
		var pCnt int64
		model.DB.Model(&model.User{}).Where("phone_login = ?", phone).Count(&pCnt)
		if pCnt > 0 {
			badRequest(c, "该手机号已注册")
			return
		}
	}

	user := model.User{
		Username:     uname,
		PhoneLogin:   phone, // 手机号即登录账号
		Phone:        phone,
		RealName:     realName,
		DepartmentID: deptID,
		Role:         model.RoleViewer, // 自助注册默认 viewer（只读）
		Status:       model.UserStatusEnabled,
		PasswordHash: model.HashPassword(req.Password),
	}
	if err := model.DB.Create(&user).Error; err != nil {
		serverError(c, err)
		return
	}

	ok(c, gin.H{"message": "注册成功，请登录", "user": userPayload(&user)})
}

// issueToken 签发 JWT（HS256，有效期 72 小时）
func issueToken(user *model.User, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(72 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(secret))
}

// findUserByLogin 按手机号登录账号或用户名定位用户（旧账号 username 兼容）。
// 手机号可作为登录账号（参考项目 a：手机号即账号）；用户名账号亦兼容。
func findUserByLogin(login string) *model.User {
	if login == "" {
		return nil
	}
	// 优先按手机号登录账号
	var u model.User
	if err := model.DB.Where("phone_login = ?", login).First(&u).Error; err == nil {
		return &u
	}
	// 兼容：username 的既有账号
	if err := model.DB.Where("username = ?", login).First(&u).Error; err == nil {
		return &u
	}
	return nil
}

// userPayload 构建用户信息（含人事核心字段/头像/工号）
func userPayload(user *model.User) gin.H {
	return gin.H{
		"id":             user.ID,
		"username":       user.Username,
		"role":           user.Role,
		"real_name":      user.RealName,
		"phone":          user.Phone,
		"phone_login":    user.PhoneLogin,
		"phone_verified": user.PhoneVerified,
		"email":          user.Email,
		"department_id":  user.DepartmentID,
		"status":         user.Status,
		"center_lat":     user.CenterLat,
		"center_lng":     user.CenterLng,
		"work_no":        user.WorkNo,
		"avatar":         user.Avatar,
		"gender":         user.Gender,
		"id_card":        user.IDCard,
		"address":        user.Address,
		"education":      user.Education,
		"engineer_level": user.EngineerLevel,
	}
}

// GetUserInfo 获取当前登录用户信息
func GetUserInfo(c *gin.Context) {
	userID := c.GetUint("user_id")

	var user model.User
	if err := model.DB.First(&user, userID).Error; err != nil {
		notFound(c, "用户不存在")
		return
	}

	ok(c, gin.H{
		"user":            userPayload(&user),
		"enabled_modules": EnabledModuleList(),
	})
}
