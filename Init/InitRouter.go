package Init

import (
	"Go_Project/api"
	"Go_Project/common/middleware"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

var userService service.UserService
var base api.BaseService

func Routers() *gin.Engine {
	Router := gin.Default()
	Router.Use(middleware.CrosHandler())
	Router.MaxMultipartMemory = 8 << 20 // 8 MiB
	Router.StaticFS("../static/headImags", http.Dir("headImags"))
	v1 := Router.Group("/api/v1/user")
	v1.POST("/register", base.Register)
	// 登录接口
	v1.POST("/login", userService.Login)
	v2 := v1.Group("/base").Use(middleware.JWTAuth(), middleware.CasbinController())
	// 通过id查询用户接口
	v2.GET("/:id", userService.QueryUserService)
	// 查询用户接口
	v2.GET("user", userService.QueryAll)
	// 通过id删除用户
	v2.DELETE("/:id", userService.Delete)
	// 重置密码
	v2.POST("/resetPwd", base.ResetPassword)
	// 上传头像
	v2.PUT("/updateImage/:id", userService.UpLoadHeaderImage)
	return Router
}
