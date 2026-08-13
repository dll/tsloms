package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

// Auth JWT 鉴权中间件
// 从 Authorization 头提取 Bearer token，验证签名和有效期
// 仅接受 HS256 算法，角色以数据库为准
func Auth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		// 去除 Bearer 前缀
		tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")

		// 解析并验证 JWT
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			// 仅接受 HS256 算法，防止算法混淆攻击
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok || t.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// 提取 claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
			c.Abort()
			return
		}

		// 提取用户 ID
		userID, ok := claims["user_id"].(float64)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
			c.Abort()
			return
		}

		// 实时校验用户是否存在
		var user model.User
		if err := model.DB.First(&user, uint(userID)).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			c.Abort()
			return
		}

		// 将用户信息注入上下文
		c.Set("user_id", user.ID)
		c.Set("user_role", user.Role)
		c.Next()
	}
}

// RequireAdmin 管理员权限校验中间件
// 必须在 Auth 中间件之后使用
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("user_role")
		if role != model.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "需要管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireOperator 运维人员权限校验中间件
// 管理员和运维人员可通过，查看人员不可
func RequireOperator() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("user_role")
		if role != model.RoleAdmin && role != model.RoleOperator {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "需要运维权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}
