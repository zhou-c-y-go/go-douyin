package Init

import (
	"Go_Project/api"
	"Go_Project/common/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
)

var userController api.UserController
var videoController = api.VideoController{}

func Routers() *gin.Engine {
	Router := gin.Default()
	Router.Use(middleware.CrosHandler())
	Router.Use(middleware.TraceMiddleware())
	Router.MaxMultipartMemory = 8 << 20 // 8 MiB
	Router.StaticFS("../static/headImags", http.Dir("headImags"))
	v1 := Router.Group("/api/v1/user")
	v1.POST("/register", userController.Register)
	v1.GET("/video/feed", videoController.GetFeedStream)
	v1.PUT("/video/repair-duration", videoController.RepairDuration)
	// 登录接口
	v1.POST("/login", userController.Login)
	authGroup := Router.Group("/api/v1/user").Use(middleware.JWTAuth())
	{
		authGroup.POST("/video/publish", videoController.UploadVideo)
		// 个人主页接口 (这样写，JWTAuth 绝对在 GetUserProfile 之前执行！)
		authGroup.GET("/profile", userController.GetUserProfile)
		authGroup.PUT("/update", userController.UpdateUserInfo)
		authGroup.POST("/avatar", userController.UploadHeaderImage)
	}
	// 管理员端口
	//v2 := Router.Group("/admin").Use(middleware.CasbinController())
	//// 通过id查询用户接口
	//v2.GET("/:id", userService.QueryUserService)
	//// 查询用户接口
	//v2.GET("user", userService.QueryAll)
	//// 通过id删除用户
	//v2.DELETE("/:id", userService.Delete)
	//// 重置密码
	//v2.POST("/resetPwd", base.ResetPassword)
	//// 上传头像
	//v2.PUT("/updateImage/:id", userService.UploadAvatar)
	return Router
}
