package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// RequirePerm 功能权限校验中间件
// 必须在 Auth 中间件之后使用。
// 判定逻辑：取当前用户的有效权限集合（用户级覆写，否则取角色默认权限），
//
//	不存在指定权限编码则拒绝（403）。
func RequirePerm(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("user_id")
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		perms, err := model.EffectivePermissions(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "permission query failed", "message": "权限查询失败"})
			c.Abort()
			return
		}
		if !perms[perm] {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "无此功能权限: " + perm})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequirePerms 需要多个权限中的任一即可（OR）
func RequirePerms(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("user_id")
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		userPerms, err := model.EffectivePermissions(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "permission query failed", "message": "权限查询失败"})
			c.Abort()
			return
		}
		for _, p := range perms {
			if userPerms[p] {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "无相应功能权限"})
		c.Abort()
	}
}
