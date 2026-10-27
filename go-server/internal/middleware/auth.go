// middleware 包定义 HTTP 中间件
package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rocuae/importsshkey/go-server/internal/config"
)

const (
	// maxAuthFailures 单个 IP 在窗口期内允许的失败次数。
	maxAuthFailures = 10
	// authWindow 失败计数的时间窗口。
	authWindow = time.Minute
)

type authFailure struct {
	// count 窗口期内的失败次数
	count int
	// reset 计数失效时间
	reset time.Time
}

var (
	// authFailuresMu 保护 authFailures 的并发访问
	authFailuresMu sync.Mutex
	// authFailures 按客户端 IP 记录失败计数
	authFailures = make(map[string]*authFailure)
)

// AdminAuth 管理员认证中间件
// 验证请求头中的 Authorization: Bearer <token>
// 如果 ADMIN_TOKEN 未配置，返回 403 禁止写操作
// 参数：
//   - cfg: 应用配置
//
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

		clientIP := c.ClientIP()
		if blocked(clientIP) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many failed authentication attempts",
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
			recordFailure(clientIP)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid Authorization format. Use: Bearer <token>",
			})
			c.Abort()
			return
		}

		if !constantTimeEqual(parts[1], cfg.AdminToken) {
			recordFailure(clientIP)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			c.Abort()
			return
		}

		clearFailures(clientIP)
		c.Next()
	}
}

// constantTimeEqual 通过定长摘要比较 Token，避免长度与内容时序泄露。
func constantTimeEqual(actual, expected string) bool {
	actualHash := sha256.Sum256([]byte(actual))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(actualHash[:], expectedHash[:]) == 1
}

// blocked 判断 IP 是否已超过失败阈值，并顺带清理过期记录。
func blocked(ip string) bool {
	authFailuresMu.Lock()
	defer authFailuresMu.Unlock()
	entry, ok := authFailures[ip]
	if !ok {
		return false
	}
	if time.Now().After(entry.reset) {
		delete(authFailures, ip)
		return false
	}
	return entry.count >= maxAuthFailures
}

// recordFailure 累加失败次数，窗口过期后重新计数。
func recordFailure(ip string) {
	authFailuresMu.Lock()
	defer authFailuresMu.Unlock()
	now := time.Now()
	entry, ok := authFailures[ip]
	if !ok || now.After(entry.reset) {
		authFailures[ip] = &authFailure{count: 1, reset: now.Add(authWindow)}
		return
	}
	entry.count++
}

// clearFailures 在认证成功后移除该 IP 的失败记录。
func clearFailures(ip string) {
	authFailuresMu.Lock()
	defer authFailuresMu.Unlock()
	delete(authFailures, ip)
}

// BodyLimit 限制请求体大小，超过时返回 413
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "Request body too large",
			})
			c.Abort()
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// BodyTooLarge 判断错误是否为请求体超限
func BodyTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

// CORS CORS 中间件
// 参数：
//   - allowedOrigins: 允许的来源（逗号分隔，* 表示全部）
//
// 返回：
//   - gin.HandlerFunc: 中间件函数
func CORS(allowedOrigins string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// 通配模式下无需回显 Origin；白名单模式必须回显并声明 Vary。
		if allowedOrigins == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			origins := strings.Split(allowedOrigins, ",")
			for _, o := range origins {
				if strings.TrimSpace(o) == origin {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Vary", "Origin")
					break
				}
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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
