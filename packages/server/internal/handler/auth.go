package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
	"golang.org/x/crypto/bcrypt"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 用户名密码登录，返回 JWT token
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "用户名和密码不能为空")
		return
	}

	// 查找用户
	var user model.User
	if err := model.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		unauthorized(c, "用户名或密码错误")
		return
	}

	// 验证密码（bcrypt）
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		unauthorized(c, "用户名或密码错误")
		return
	}

	// 签发 JWT
	cfg := config.Get()
	token, err := issueToken(&user, cfg.JWTSecret)
	if err != nil {
		serverError(c, err)
		return
	}

	ok(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
			"phone":    user.Phone,
		},
	})

	// 记录登录操作日志
	c.Set("op_username", user.Username)
	recordOperation(c, model.OpLogin, "auth/login", "用户登录")
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

// GetUserInfo 获取当前登录用户信息
func GetUserInfo(c *gin.Context) {
	userID := c.GetUint("user_id")

	var user model.User
	if err := model.DB.First(&user, userID).Error; err != nil {
		notFound(c, "用户不存在")
		return
	}

	ok(c, gin.H{
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
			"phone":    user.Phone,
		},
	})
}

