package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func CrosHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method

		// 1. 设置允许访问所有域
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Origin", "*")

		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")

		// 💡 核心修复 A：把 X-Trace-Id 和 x-trace-id 强行塞进允许前端携带的请求头白名单里！
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token, session, X_Requested_With, Accept, Origin, Host, Connection, Accept-Encoding, Accept-Language, DNT, X-CustomHeader, Keep-Alive, User-Agent, X-Requested-With, If-Modified-Since, Cache-Control, Content-Type, Pragma, token, openid, opentoken, X-Trace-Id, x-trace-id")

		// 2. 暴露给前端可见的响应头（确保前端无感续签能拿到 New-Token，链路跟踪能拿到 X-Trace-Id）
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type, Expires, Last-Modified, Pragma, FooBar, X-Trace-Id, x-trace-id, New-Token, New-Refresh-Token, new-token, new-expired-at")

		c.Header("Access-Control-Max-Age", "172800")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Set("content-type", "application/json")

		// 💡 核心修复 B：如果是浏览器的 OPTIONS 预检请求，给够权限，直接 Abort 截断返回，绝不往下传！
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent) // 204 或者 200 都可以，直接光速返回
			return
		}

		// 处理真正的前端业务请求（GET/POST/PUT/DELETE）
		c.Next()
	}
}
