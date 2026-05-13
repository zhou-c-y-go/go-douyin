package middleware

import (
	"Go_Project/common/model/response"
	"Go_Project/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"strconv"
	"strings"
	"time"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Request.Header.Get("x-token")
		if token == "" {
			response.Fail(c, response.ERROR, "非法登录")
			c.Abort()
			return
		}
		j := utils.NewJWT()
		claims, err := j.ParseToken(token)
		if err != nil {
			if strings.Contains(err.Error(), "expired") {
				response.Fail(c, response.ERROR, "token已经过期")
				c.Abort()
				return
			}
			response.Fail(c, response.ERROR, err.Error())
		}
		if claims.ExpiresAt.Unix() >= time.Now().Unix() {
			claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Hour))
			newToken, _ := j.CreateTokenByOldToken(token, *claims)
			newClaim, _ := j.ParseToken(newToken)
			c.Header("new-token", newToken)
			c.Header("new-expired-at", strconv.FormatInt(newClaim.ExpiresAt.Unix(), 10))
		}
		c.Set("claim", claims)
		c.Next()
	}
}
