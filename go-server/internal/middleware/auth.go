// middleware 包定义 HTTP 中间件
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/importsshkey/iskey-server/internal/config"
)

// AdminAuth 管理员认证中间件
// 验证请求头中的 Authorization: Bearer <token>
// 如果 ADMIN_TOKEN 未配置，返回 403 禁止写操作
// 参数：
//   - cfg: 应用配置
// 返回：
//   - gin.HandlerFunc: 中间件函数
func AdminAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果未配置 ADMIN_TOKEN，禁止写操作
		if cfg.AdminToken == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Write operations disabled: ADMIN_TOKEN not configured",
			})
			c.Abort()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing Authorization header",
			})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid Authorization format. Use: Bearer <token>",
			})
			c.Abort()
			return
		}

		token := parts[1]
		if token != cfg.AdminToken {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CORS CORS 中间件
// 参数：
//   - allowedOrigins: 允许的来源（逗号分隔，* 表示全部）
// 返回：
//   - gin.HandlerFunc: 中间件函数
func CORS(allowedOrigins string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if allowedOrigins == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			origins := strings.Split(allowedOrigins, ",")
			for _, o := range origins {
				if strings.TrimSpace(o) == origin {
					c.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Logger 请求日志中间件
// 返回：
//   - gin.HandlerFunc: 中间件函数
func Logger() gin.HandlerFunc {
	return gin.Logger()
}
