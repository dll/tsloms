package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

// ok 统一成功响应，格式: { code: 0, msg: "success", data: {...} }
func ok(c *gin.Context, data gin.H) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

// Health 健康检查（公开接口，用于 Nginx / 探活）
func Health(c *gin.Context) {
	ok(c, gin.H{
		"status":  "ok",
		"service": "tsloms-server",
		"env":     config.Get().AppEnv,
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

// forbidden 403（含模块未启用/权限不足）
func forbidden(c *gin.Context, message string) {
	fail(c, http.StatusForbidden, "forbidden", message)
}

// notFound 资源不存在
func notFound(c *gin.Context, message string) {
	fail(c, http.StatusNotFound, "not_found", message)
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

// 统一分页参数约束
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// paginate 统一解析并校验分页参数：page≥1，1≤page_size≤100。
// 非法/越界值回退默认，避免超大 Limit/Offset 拖垮查询。
func paginate(c *gin.Context) (uint, uint) {
	page, _ := parseUint(c.DefaultQuery("page", "1"))
	pageSize, _ := parseUint(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// RoleIsOperator 判断角色是否具备运维操作能力（管理人员亦具备运维权限）
func RoleIsOperator(role string) bool {
	return role == model.RoleAdmin || role == model.RoleOperator
}

// isOperator 判断当前用户是否为运维人员（管理员也具有运维权限）
func isOperator(c *gin.Context) bool {
	return RoleIsOperator(c.GetString("user_role"))
}
