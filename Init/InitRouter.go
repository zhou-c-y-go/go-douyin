package Init

import (
	"Go_Project/api"
	"Go_Project/common/middleware"
	"Go_Project/common/repository"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

func Routers() *gin.Engine {
	// 1. 注入持久层
	userRepo := repository.NewUserRepository()
	videoRepo := repository.NewVideoRepository()
	commentRepo := repository.NewCommentRepository()
	likeRepo := repository.NewLikeRepository()
	favRepo := repository.NewFavoriteRepository()
	// 2. 注入业务层
	userService := service.NewUserService(userRepo)
	videoService := service.NewVideoService(videoRepo, userRepo, likeRepo, favRepo) // video 依赖 user 模块拉作者信息
	commentService := service.NewCommentService(commentRepo, userRepo)              // comment 依赖 user 模块拉卡片信息
	likeService := service.NewLikeService(likeRepo)
	favorService := service.NewFavorService(favRepo)
	// 3. 注入服务层
	userController := api.NewUserController(userService, likeService, favorService, videoService)
	videoController := api.NewVideoController(videoService)
	commentController := api.NewCommentController(commentService)
	likeController := api.NewLikeController(likeService)
	favoriteController := api.NewFavoriteController(favorService)
	Router := gin.Default()
	Router.Use(middleware.CrosHandler())
	Router.Use(middleware.TraceMiddleware())
	Router.MaxMultipartMemory = 8 << 20 // 8 MiB
	Router.StaticFS("../static/headImags", http.Dir("headImags"))
	v1 := Router.Group("/api/v1/user")
	v1.POST("/register", userController.Register)
	v1.GET("/video/feed", videoController.GetFeedStream)
	v1.PUT("/video/repair-duration", videoController.RepairDuration)
	v1.GET("/user/info", userController.GetPublicUserInfo)
	// 登录接口
	v1.POST("/login", userController.Login)

	Group1 := Router.Group("/api/v1/user").Use(middleware.JWTAuthOptional())
	{ // 视频详细信息
		Group1.GET("/video/detail", videoController.GetVideoDetail)
	}
	Group2 := Router.Group("/api/v1/user").Use(middleware.JWTAuth())
	{
		Group2.GET("/profile", userController.GetUserProfile)
		Group2.PUT("/update", userController.UpdateUserInfo)
		Group2.POST("/avatar", userController.UploadHeaderImage)
		Group2.POST("/video/get-presigned-url", videoController.GetPresignedUploadURL)
		Group2.POST("/video/publish", videoController.UploadVideo)
		Group2.POST("/video/init-multipart", videoController.InitMultipart)
		Group2.POST("/video/complete-multipart", videoController.CompleteMultipart)
		Group2.GET("/video/user/list", videoController.GetUserVideoList)
		Group2.GET("/comment/tree", commentController.GetVideoCommentTree)
		Group2.POST("/comment", commentController.CreateComment)
		Group2.POST("/like", likeController.ToggleLike)
		Group2.POST("/favorite", favoriteController.ToggleFavorite)
		Group2.POST("/calibrate", likeController.CalibrateVideoCounts)
		Group2.GET("/video/user/like-list", videoController.GetUserLikedVideoList)
		Group2.GET("/video/user/favor-list", videoController.GetUserFavoriteVideoList)
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
