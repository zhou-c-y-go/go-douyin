package middleware

import (
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/utils"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

var casbin utils.CasbinService

func CasbinController() gin.HandlerFunc {
	return func(c *gin.Context) {
		claim, _ := utils.GetClaim(c)
		if claim == nil {
			response.Fail(c, response.ERROR, "请您登录")
			global.SugaredLogger.Info("用户没有登录")
			c.Abort()
			return
		}
		sub := strconv.Itoa(int(claim.AuthorityId))
		act := c.Request.Method
		path := c.Request.URL.Path
		obj := strings.TrimPrefix(path, global.GLOB_CONFIG.System.RouterPrefix)
		e := casbin.InitConfig()
		err := e.LoadPolicy()
		if err != nil {
			global.SugaredLogger.Errorw("角色规则加载失败:", "err", err.Error())
			return
		}
		success, _ := e.Enforce(sub, obj, act)
		if !success {
			response.Fail(c, response.ERROR, "权限不足")
			global.SugaredLogger.Infow("权限不足:", "err", err.Error())
			c.Abort()
			return
		}
		c.Next()
	}
}
