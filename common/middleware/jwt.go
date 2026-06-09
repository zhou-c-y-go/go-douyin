package middleware

import (
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/utils"
	"errors"
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

		// 1. 提取大厂标准的 Authorization 头
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			response.Fail(c, response.ERROR, "未登录，请先登录")
			global.LogCtx(c).Errorln("token解析失败: 请求头未携带 Authorization")
			c.Abort()
			return
		}

		// 2. 将 "Bearer <token>" 按空格切分，提取真正的 Token 字符串
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			response.Fail(c, response.ERROR, "请求头格式错误，应为 Bearer <token>")
			global.LogCtx(c).Errorln("请求头格式错误，应为 Bearer <token>")
			c.Abort()
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			response.Fail(c, response.ERROR, "非法登录，请先登录")
			global.LogCtx(c).Errorf("%s 非法登录: token 为空\n", c.ClientIP())
			c.Abort()
			return
		}

		// 3. 实例化并直接解析 Token（不需要在之前盲查一次 Redis 啦）
		j := utils.NewJWT()
		claims, err := j.ParseToken(token)
		if err != nil {
			// 💡 修复：如果 Token 解析失败（伪造或过期），必须立刻阻断，不能继续往下走！
			response.Fail(c, response.ERROR, "登录已过期或凭证无效，请重新登录")
			global.LogCtx(c).Errorw("token解析失败", "err", err)
			c.Abort()
			return
		}

		// 4. 使用解密出来的 claims.ID（或 claims.Id，请与你结构体大小写对齐）去拼 Redis 键
		// 完美对齐你在 LoginNext 里设置的：fmt.Sprintf("UserToken:%d", user.ID)
		redisKey := fmt.Sprintf("UserToken:%d", claims.Id)
		println(redisKey)
		cachedToken, err := global.GVA_REDIS.Get(c.Request.Context(), redisKey).Result()
		if errors.Is(err, redis.Nil) {
			response.Fail(c, response.ERROR, "未找到token")
			c.Abort() // 💡 必须阻断，不允许继续往下执行
			return
		} else if err != nil {
			global.LogCtx(c).Errorw("Redis校验Token异常", "err", err)
			response.Fail(c, response.ERROR, "Redis校验Token异常")
			c.Abort() // 💡 异常情况也要及时阻断
			return
		}

		global.LogCtx(c).Infoln("内存击中，拿到了用户token")

		// 5. 校验当前 Token 是否与 Redis 里的最新 Token 一致（单点登录，防多设备同时在线）
		if cachedToken != token {
			c.JSON(401, gin.H{"code": 401, "msg": "您的账号已在其他设备登录"})
			c.Abort()
			return
		}

		// 6. 无感续签逻辑
		now := time.Now().Unix()
		expiresAt := claims.ExpiresAt.Unix()
		if expiresAt > now && (expiresAt-now) < 1800 {
			claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Hour))
			newToken, err := j.CreateTokenByOldToken(token, *claims)

			// 💡 修复：这里原本写成了 if err != nil，生成新 token 成功时 err 应该为 nil
			if err == nil {
				// 续签成功，同步更新 Redis 的单点登录凭证
				global.GVA_REDIS.Set(c.Request.Context(), redisKey, newToken, 24*time.Hour)
				// 将新的 Token 塞进响应头，让前端静默更新
				c.Header("new-token", newToken)
				c.Header("new-expired-at", strconv.FormatInt(claims.ExpiresAt.Unix(), 10))
				global.LogCtx(c).Infof("用户[%d]触发无感续签，Redis已同步更新", claims.ID)
			} else {
				global.LogCtx(c).Errorw("无感续签生成新Token失败", "err", err)
			}
		}

		// 7. 写入上下文，放行进入 Controller
		c.Set("claim", claims)
		c.Next()
	}
}

func JWTAuthOptional() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			// 探针 1
			global.LogCtx(c).Warnln("🛡️ [软鉴权探针] 请求头里空空如也，直接降级为游客")
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			// 探针 2
			global.LogCtx(c).Warnln("🛡️ [软鉴权探针] Token 格式不是 Bearer 规范，降级为游客")
			c.Next()
			return
		}

		token := strings.TrimSpace(parts[1])
		j := utils.NewJWT()
		claims, err := j.ParseToken(token)
		if err != nil {
			// 探针 3
			global.LogCtx(c).Errorw("🛡️ [软鉴权探针] Token 解析硬性流产了！", "err", err)
			c.Next()
			return
		}

		redisKey := fmt.Sprintf("UserToken:%d", claims.Id)
		cachedToken, err := global.GVA_REDIS.Get(c.Request.Context(), redisKey).Result()
		if err != nil {
			// 探针 4
			global.LogCtx(c).Errorw("🛡️ [软鉴权探针] Redis 账本里查无此 Token 或异常", "err", err)
			c.Next()
			return
		}

		if cachedToken != token {
			// 探针 5
			global.LogCtx(c).Warnln("🛡️ [软鉴权探针] 发现单点登录冲突，被其他设备顶号了，降级为游客")
			c.Next()
			return
		}

		// 🎯 只有冲破以上所有重围，才有资格点亮这里！
		global.LogCtx(c).Infof("🛡️ [软鉴权探针] 🎉 恭喜！搜身成功，注入身份 ID: %d", claims.Id)
		c.Set("claim", claims)
		c.Next()
	}
}
