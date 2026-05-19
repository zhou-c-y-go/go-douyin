package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/gin-gonic/gin"
)

// 简单的十六进制随机字符串生成器
func generateTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 尝试从前端请求头里获取 TraceID（万一前端有自己的全网追踪）
		traceID := c.GetHeader("X-Trace-Id")
		if traceID == "" {
			traceID = generateTraceID() // 没带就后端签发
		}

		// 2. 注入到 Gin 上下文，方便在 Gin 内部流转
		c.Set("traceId", traceID)

		// 3. 💡 最地道的大厂写法：同时注入到标准 Go 核心 Request Context 中
		ctx := context.WithValue(c.Request.Context(), "traceId", traceID)
		c.Request = c.Request.WithContext(ctx)

		// 4. 将 TraceID 写入响应头，返还给前端（前端报错时可以展示出来）
		c.Header("X-Trace-Id", traceID)

		c.Next()
	}
}
