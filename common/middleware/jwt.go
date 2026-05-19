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
			newToken, err := j.CreateTokenByOldToken(token, *claims)
			if err != nil {
				redisKey := fmt.Sprintf("UserToken:%d", claims.ID)
				global.GVA_REDIS.Set(c.Request.Context(), redisKey, newToken, 24*time.Hour)
				// 2. 将新的 Token 塞进响应头，让前端静默更新
				c.Header("new-token", newToken)
				c.Header("new-expired-at", strconv.FormatInt(claims.ExpiresAt.Unix(), 10))
				global.SugaredLogger.Infof("用户[%d]触发无感续签，Redis已同步更新", claims.ID)
			} else {
				global.SugaredLogger.Errorw("无感续签生成新Token失败", "err", err)
			}
		}
		c.Set("claim", claims)
		c.Next()
	}
}
