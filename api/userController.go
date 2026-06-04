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
)

var userService service.UserService

type UserController struct{}

// Register 注册接口
func (b *UserController) Register(c *gin.Context) {
	var r request.Register
	ctx := c.Request.Context()
	if err := utils.InitTrans("zh"); err != nil {
		global.LogCtx(c).Errorw("翻译器初始化失败:", "err", err)
	}
	if err := c.ShouldBind(&r); err != nil {
		var errs validator.ValidationErrors
		ok := errors.As(err, &errs)
		if !ok {
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":          response.ERROR,
			"error-message": errs.Translate(utils.Trans),
			"data":          nil,
		})
		global.LogCtx(c).Errorw("参数绑定错误", "err", err)
		return
	}
	user := &pojo.User{
		Username:  r.Username,
		Password:  r.Password,
		Email:     r.Email,
		Telephone: r.Telephone,
		Status:    r.Status,
	}
	userService := service.UserService{}
	userReturn, err := userService.Register(ctx, *user)
	if err != nil {
		global.LogCtx(c).Error("注册失败!", zap.Error(err))
		response.Fail(c, response.ERROR, "注册失败!")
		return
	}
	response.Success(c, userReturn)
}

// ResetPassword 密码重置接口
func (b *UserController) ResetPassword(c *gin.Context) {
	var user pojo.User
	ctx := c.Request.Context()
	err := c.ShouldBindJSON(&user)
	if err != nil {
		global.LogCtx(c).Errorw("参数绑定错误", "err", err)
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	err = userService.ResetPassword(ctx, uint(user.ID))
	if err != nil {
		global.LogCtx(c).Error("密码重置失败!", zap.Error(err))
		response.Fail(c, response.ERROR, "重置失败"+err.Error())
		return
	}
	response.Success(c, "重置成功")
}

// Login 登录
func (s *UserController) Login(c *gin.Context) {
	var l request.Login
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(400, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}
	ctx := c.Request.Context()
	global.LogCtx(ctx).Infof("📥 [Controller] 收到用户 [%s] 的登录请求", l.Username)
	userService := service.UserService{}
	token, user, err := userService.LoginService(ctx, l.Username, l.Password)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Controller] 用户 [%s] 登录失败: %v", l.Username, err)
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	// 4. 打包发货：此时确保 Service 一路全绿通过后，才吐出唯一一次 Success，完美解决逻辑冲突 Bug！
	global.LogCtx(ctx).Infof("📤 [Controller] 用户 [%s] 登录成功，Token 与 Redis 分发完毕", user.Username)
	response.Success(c, token)
}

// GetUserProfile 获取个人空间所需的信息
func (s *UserController) GetUserProfile(c *gin.Context) {
	// 1. 【第一层 - JWT】从上下文中获取 jwt.go 解析出的 claims
	claimInterface, exists := c.Get("claim")
	if !exists {
		response.Fail(c, response.ERROR, "获取jwt失败")
		return
	}

	// 断言为你在 jwt.go 中定义的 *request.CustomClaims
	claims, ok := claimInterface.(*request.CustomClaims)
	if !ok {
		response.Fail(c, response.ERROR, "系统内部错误：JWT 结构异常")
		return
	}
	ctx := c.Request.Context()
	global.LogCtx(ctx).Infof("📥 [Controller] 收到用户 ID [%d] 的主页资料调取请求", claims.Id)
	userService := service.UserService{}
	userProfile, err := userService.GetUserProfileService(ctx, claims.Id, claims.UserName)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Controller] 获取用户主页资料失败: %v", err)
		c.JSON(404, gin.H{"code": 404, "msg": err.Error()}) // 保持你原有的 404 格式
		return
	}
	response.Success(c, userProfile)
}

// UpdateUserInfo 更新用户个人信息
func (api *UserController) UpdateUserInfo(c *gin.Context) {
	// 1. 安全验证
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
	// 2. 绑定前端发过来的修改资料参数
	var reqUser pojo.User
	if err := c.ShouldBindJSON(&reqUser); err != nil {
		global.LogCtx(c.Request.Context()).Errorw("修改用户信息参数绑定失败", "err", err)
		response.Fail(c, response.ERROR, "参数格式错误")
		return
	}
	ctx := c.Request.Context()
	// 3. 强行锁定 ID，防止越权恶意修改别人资料
	reqUser.ID = claims.Id
	// 4. 传唤 Service 逻辑篮子去更新 MySQL 并淘汰 Redis 缓存
	userService := service.UserService{}
	err := userService.UpdateUserInfoService(ctx, &reqUser)
	if err != nil {
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
	userID := int(claims.Id) // 转换为你业务层需要的 int 类型

	file, _ := c.FormFile("head-img")
	fileObj, err := file.Open()
	if err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	defer func(fileObj multipart.File) {
		err := fileObj.Close()
		if err != nil {
			response.Fail(c, response.ERROR, err.Error())
		}
	}(fileObj)
	ctx := c.Request.Context()
	global.LogCtx(ctx).Infof("📥 [Controller] 用户 ID [%d] 正在发起 MinIO 头像上传", userID)
	var u pojo.User
	err = userService.UpLoadHeaderImage(ctx, &u, userID, file, fileObj)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Controller] 用户 [%d] 头像上传业务出错: %v", userID, err)
		response.Fail(c, response.ERROR, err.Error())
	} else {
		global.LogCtx(ctx).Infof("📤 [Controller] 用户 [%d] 全链路头像上传、回源、淘汰缓存完成！", userID)
		response.Success(c, response.OK)
	}
}

// GetPublicUserInfo ── 🌐 对应前端：request.get('/user/info?id=xxx')
// 这是一个纯粹的公开大闸门，只认 URL 里的 id，不查 JWT！
func (s *UserController) GetPublicUserInfo(c *gin.Context) {
	// 1. 抓取 URL 上挂载的目标创作者 ID
	idStr := c.Query("id")
	targetId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || targetId <= 0 {
		response.Fail(c, response.ERROR, "无法锁定该创作者的坐标")
		return
	}

	ctx := c.Request.Context()
	userService := service.UserService{} // 实例化你的用户服务

	// 2. 传唤服务层起飞（咱们复用你原有的 GetUserProfileService 即可，只要不传敏感信息就行）
	// 注意：因为是看别人，所以 username 我们可以传空字符串，让底层自己查
	userProfile, err := userService.GetUserProfileService(ctx, targetId, "")
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Controller] 捞取创作者 [%d] 公开名片失败: %v", targetId, err)
		response.Fail(c, response.ERROR, "该创作者可能已注销或隐藏了空间")
		return
	}

	// 3. 向前端倾泻数据洪流
	response.Success(c, userProfile)
}
