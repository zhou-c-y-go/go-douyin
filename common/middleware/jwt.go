package middleware

import (
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/utils"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"strconv"
	"strings"
	"time"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Request.Header.Get("x-token")
		if token == "" {
			response.Fail(c, response.ERROR, "未登录，请先登录")
			c.Abort()
			return
		}
		j := utils.NewJWT()
		claims, err := j.ParseToken(token)

		redisKey := fmt.Sprintf("UserToken:%d", claims.ID)
		cachedToken, err := global.GVA_REDIS.Get(c.Request.Context(), redisKey).Result()
		global.LogCtx(c).Infoln("内存集中，拿到了用户token")
		if err == redis.Nil {
			response.Fail(c, response.ERROR, "登录已过期，请重新登录")
		} else if strings.Contains(err.Error(), "expired") {
			global.LogCtx(c).Errorw("Redis校验Token异常", "err", err)
			response.Fail(c, response.ERROR, "Redis校验Token异常")
			c.Abort()
			return
		}
		if cachedToken != token {
			c.JSON(401, gin.H{"code": 401, "msg": "您的账号已在其他设备登录"})
			c.Abort()
			return
		}
		// 更新token
		now := time.Now().Unix()
		expiresAt := claims.ExpiresAt.Unix()
		if expiresAt > now && (expiresAt-now) < 1800 {
			claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Hour))
			newToken, err := j.CreateTokenByOldToken(token, *claims)
			if err != nil {
				redisKey := fmt.Sprintf("UserToken:%d", claims.ID)
				global.GVA_REDIS.Set(c.Request.Context(), redisKey, newToken, 24*time.Hour)
				// 2. 将新的 Token 塞进响应头，让前端静默更新
				c.Header("new-token", newToken)
				c.Header("new-expired-at", strconv.FormatInt(claims.ExpiresAt.Unix(), 10))
				global.LogCtx(c).Infof("用户[%d]触发无感续签，Redis已同步更新", claims.ID)
			} else {
				global.LogCtx(c).Errorw("无感续签生成新Token失败", "err", err)
			}
		}
		c.Set("claim", claims)
		c.Next()
	}
}
