package api

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/service"
	"Go_Project/utils"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
	"mime/multipart"
	"net/http"
	"strconv"
)

type UserController struct {
	userService  service.UserService
	likeService  service.LikeService
	favorService service.FavorService
	videoService service.VideoService
}

func NewUserController(us service.UserService, ls service.LikeService, fs service.FavorService, vs service.VideoService) *UserController {
	return &UserController{
		userService:  us,
		likeService:  ls,
		favorService: fs,
		videoService: vs,
	}
}

func (api *UserController) Register(c *gin.Context) {
	var r request.Register
	ctx := c.Request.Context()
	if err := utils.InitTrans("zh"); err != nil {
		global.LogCtx(c).Errorw("翻译器初始化失败:", "err", err)
	}
	if err := c.ShouldBind(&r); err != nil {
		var errs validator.ValidationErrors
		if ok := errors.As(err, &errs); !ok {
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":          response.ERROR,
			"error-message": errs.Translate(utils.Trans),
			"data":          nil,
		})
		return
	}
	user := &pojo.User{
		Username: r.Username, Password: r.Password,
		Email: r.Email, Telephone: r.Telephone, Status: r.Status,
	}

	userReturn, err := api.userService.Register(ctx, *user)
	if err != nil {
		global.LogCtx(c).Error("注册失败!", zap.Error(err))
		response.Fail(c, response.ERROR, "注册失败!")
		return
	}
	response.Success(c, userReturn)
}

func (api *UserController) ResetPassword(c *gin.Context) {
	var user pojo.User
	ctx := c.Request.Context()
	if err := c.ShouldBindJSON(&user); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	if err := api.userService.ResetPassword(ctx, uint(user.ID)); err != nil {
		response.Fail(c, response.ERROR, "重置失败"+err.Error())
		return
	}
	response.Success(c, "重置成功")
}

func (api *UserController) Login(c *gin.Context) {
	var l request.Login
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	ctx := c.Request.Context()
	token, _, err := api.userService.LoginService(ctx, l.Username, l.Password)
	if err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	response.Success(c, token)
}

func (api *UserController) GetUserProfile(c *gin.Context) {
	claimInterface, exists := c.Get("claim")
	if !exists {
		response.Fail(c, response.ERROR, "获取jwt失败")
		return
	}
	claims, ok := claimInterface.(*request.CustomClaims)
	if !ok {
		response.Fail(c, response.ERROR, "系统内部错误：JWT 结构异常")
		return
	}
	ctx := c.Request.Context()

	// 1. 捞取静态资料（走 Redis 拦截）
	userProfile, err := api.userService.GetUserProfileService(ctx, claims.Id, claims.UserName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}

	// =========================================================================
	// 🎯【终极降维打击】：用一发内存读取，直接打包带回四大核心计数！
	// =========================================================================
	counters, err := api.likeService.GetUserAllCounters(ctx, claims.Id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "计数账本读取异常"})
		return
	}

	// 2. 闪电组装，前端完全察觉不到背后的动静分离机制
	userProfile.TotalLiked = counters["TotalLike"]
	userProfile.FavoriteCount = counters["FavorCount"]
	userProfile.WorkCount = counters["WorkCount"]

	response.Success(c, userProfile)
}
func (api *UserController) UpdateUserInfo(c *gin.Context) {
	claimInterface, exists := c.Get("claim")
	if !exists {
		response.Fail(c, response.ERROR, "未授权，无法获取当前登录状态")
		return
	}
	claims, ok := claimInterface.(*request.CustomClaims)
	if !ok {
		response.Fail(c, response.ERROR, "系统内部错误：JWT 结构异常")
		return
	}
	var reqUser pojo.User
	if err := c.ShouldBindJSON(&reqUser); err != nil {
		response.Fail(c, response.ERROR, "参数格式错误")
		return
	}
	ctx := c.Request.Context()
	reqUser.ID = claims.Id
	if err := api.userService.UpdateUserInfoService(ctx, &reqUser); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	response.Success(c, "个人信息更新成功")
}

func (api *UserController) UploadHeaderImage(c *gin.Context) {
	claimInterface, exists := c.Get("claim")
	if !exists {
		response.Fail(c, response.ERROR, "未授权，无法获取当前登录状态")
		return
	}
	claims, ok := claimInterface.(*request.CustomClaims)
	if !ok {
		response.Fail(c, response.ERROR, "系统内部错误：JWT 结构异常")
		return
	}
	userID := int(claims.Id)
	file, err := c.FormFile("head-img")
	if err != nil {
		response.Fail(c, response.ERROR, "获取图片失败")
		return
	}
	fileObj, err := file.Open()
	if err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	defer func(fileObj multipart.File) {
		_ = fileObj.Close()
	}(fileObj)

	ctx := c.Request.Context()
	var u pojo.User
	if err = api.userService.UpLoadHeaderImage(ctx, &u, userID, file, fileObj); err != nil {
		response.Fail(c, response.ERROR, err.Error())
	} else {
		response.Success(c, response.OK)
	}
}

func (api *UserController) GetPublicUserInfo(c *gin.Context) {
	idStr := c.Query("id")
	targetId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || targetId <= 0 {
		response.Fail(c, response.ERROR, "无法锁定该创作者的坐标")
		return
	}
	ctx := c.Request.Context()

	userProfile, err := api.userService.GetUserProfileService(ctx, targetId, "")
	if err != nil {
		response.Fail(c, response.ERROR, "该创作者可能已注销或隐藏了空间")
		return
	}

	// =========================================================================
	// 🎯【同理并网】：看别人的主页同样享受极致的 Redis 削峰红利
	// =========================================================================
	counters, err := api.likeService.GetUserAllCounters(ctx, targetId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "计数账本读取异常"})
		return
	}

	userProfile.TotalLiked = counters["TotalLike"]
	userProfile.FavoriteCount = counters["FavorCount"]
	userProfile.WorkCount = counters["WorkCount"]

	response.Success(c, userProfile)
}
