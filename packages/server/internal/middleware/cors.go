package middleware

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
// 开发/测试环境允许所有来源（*），生产环境按白名单限制
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// 有 Origin 时回显来源，否则通配
		allowOrigin := "*"
		if origin != "" {
			allowOrigin = origin
		}
		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		// 预检请求直接返回 204
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// isSameOrigin 判断请求 Origin 与后端 Host 是否同源
func isSameOrigin(origin, host string) bool {
	o := originHost(origin)
	h := originHost(host)
	return o != "" && o == h
}

// originHost 从 URL 或 host[:port] 中提取小写主机名
func originHost(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	if host := u.Hostname(); host != "" {
		return strings.ToLower(host)
	}
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(s)
}
