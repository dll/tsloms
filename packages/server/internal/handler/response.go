package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
)

// ok 统一成功响应，格式: { code: 0, msg: "success", data: {...} }
func ok(c *gin.Context, data gin.H) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

// created 统一创建成功响应
func created(c *gin.Context, data gin.H) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "created",
		"data": data,
	})
}

// fail 统一失败响应
func fail(c *gin.Context, status int, errCode string, message string) {
	c.JSON(status, gin.H{
		"code":  -1,
		"msg":   message,
		"error": errCode,
	})
}

// badRequest 参数错误
func badRequest(c *gin.Context, message string) {
	fail(c, http.StatusBadRequest, "bad_request", message)
}

// unauthorized 未授权
func unauthorized(c *gin.Context, message string) {
	fail(c, http.StatusUnauthorized, "unauthorized", message)
}

// notFound 资源不存在
func notFound(c *gin.Context, message string) {
	fail(c, http.StatusNotFound, "not_found", message)
}

// forbidden 无权限
func forbidden(c *gin.Context, message string) {
	fail(c, http.StatusForbidden, "forbidden", message)
}

// serverError 服务器错误
// 生产环境不向客户端回显内部错误，统一返回通用文案
func serverError(c *gin.Context, err error) {
	if err != nil {
		c.Error(err)
		if !config.Get().IsProduction() {
			fail(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
	}
	fail(c, http.StatusInternalServerError, "internal_error", "服务器内部错误")
}

// parseUint 解析路径参数为 uint
func parseUint(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	return uint(v), err
}

// isAdmin 判断当前用户是否为管理员
func isAdmin(c *gin.Context) bool {
	return c.GetString("user_role") == "admin"
}

// isOperator 判断当前用户是否为运维人员（管理员也具有运维权限）
func isOperator(c *gin.Context) bool {
	role := c.GetString("user_role")
	return role == "admin" || role == "operator"
}
