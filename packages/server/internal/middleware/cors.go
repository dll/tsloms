package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
)

// CORS 跨域中间件
// 生产环境按白名单限制（ALLOWED_ORIGINS 环境变量，逗号分隔），
// 非白名单来源不返回 CORS 头（浏览器拦截）；开发/测试环境允许所有来源。
func CORS() gin.HandlerFunc {
	cfg := config.Get()
	// 生产环境白名单（逗号分隔），如 https://admin.example.com,http://127.0.0.1:8092
	allowed := map[string]bool{}
	if cfg.IsProduction() {
		for _, o := range strings.Split(cfg.AllowedOrigins, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				allowed[strings.TrimSuffix(o, "/")] = true
			}
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		allowOrigin := ""
		if !cfg.IsProduction() {
			// 开发/测试：有 Origin 回显，无则通配
			if origin != "" {
				allowOrigin = origin
			} else {
				allowOrigin = "*"
			}
		} else if origin != "" && allowed[strings.TrimSuffix(origin, "/")] {
			// 生产：仅白名单内来源放行
			allowOrigin = origin
		}

		if allowOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
		}

		// 预检请求
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
