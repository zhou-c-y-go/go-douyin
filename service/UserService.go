package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/model/request"
	"Go_Project/common/model/response"
	repos "Go_Project/common/repository"
	"Go_Project/global"
	"Go_Project/utils"
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"strconv"
	"time"
)

type UserService struct {
}

var repository = new(repos.UserRepository)

// QueryUserService 通过id查询用户
func (s *UserService) QueryUserService(c *gin.Context) {
	var user pojo.User
	if err := c.ShouldBindJSON(&user); err != nil {
		fmt.Println(err)
		response.Fail(c, response.ERROR, "发生错误")
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	user = repository.QueryByID(id)
	if user == (pojo.User{}) {
		response.Fail(c, response.ERROR, "未找到该用户")
	} else {
		response.Success(c, user)
	}
}

// QueryAll 查询所有的用户
func (s *UserService) QueryAll(c *gin.Context) {
	var users []pojo.User
	users = repository.QueryList()
	if users == nil {
		response.Fail(c, response.ERROR, "查询失败")
	} else {
		response.Success(c, users)
	}
}

// Login 登录
func (s *UserService) Login(c *gin.Context) {
	var l request.Login
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(400, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}
	u := &pojo.User{Password: l.Password, Username: l.Username}
	user, err := repository.FindUserByIdentifier(u)
	if err != nil || user == nil {
		global.SugaredLogger.Error("登陆失败! 用户名不存在或者密码错误!", zap.Error(err))
		response.Fail(c, response.ERROR, "用户不存在或者密码错误")
		return
	}
	if user.Status != 1 {
		global.SugaredLogger.Error("登陆失败! 用户被禁止登录!")
		response.Fail(c, response.ERROR, "用户被禁止登录")
		return
	}
	s.LoginNext(c, *user)
	return
}

// LoginNext 发放令牌
func (s *UserService) LoginNext(c *gin.Context, user pojo.User) {
	// 生成JWT
	j := utils.NewJWT()
	// Claim是用户信息部分
	claim := j.CreateClaim(request.BaseClaims{
		Id:          user.ID,
		UUID:        user.UUID,
		UserName:    user.Username,
		Password:    user.Password,
		AuthorityId: user.AuthorityId,
	})
	token, err := j.CreateToken(claim)
	if err != nil {
		global.SugaredLogger.Error("获取token失败", err)
		response.Fail(c, response.ERROR, "获取token失败")
		return
	} else {
		response.Success(c, token)
	}
	ctx := c.Request.Context()
	redisKey := "JWT:" + token
	err = global.GVA_REDIS.Set(ctx, redisKey, user.Username, static.Jwt_time).Err()
	if err != nil {
		global.LogCtx(ctx).Errorw("💥 致命：Token 写入 Redis 失败！", "err", err)
		response.Fail(c, response.ERROR, "系统服务异常，请重试")
		return
	} else {
		global.LogCtx(ctx).Infof("%s token已被写入redis", redisKey)
	}
}

// Delete 根据id删除用户
func (s *UserService) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	rows := repository.Delete(id)
	if rows > 0 {
		response.Success(c, "删除成功")
		global.SugaredLogger.Infof("%#v 正在试图删除用户信息", c.ClientIP())
	} else {
		response.Fail(c, response.ERROR, "无法找到该信息")
	}

}

// Register 注册业务实现
func (s *UserService) Register(ctx context.Context, u pojo.User) (userInter pojo.User, err error) {
	var user pojo.User
	global.LogCtx(ctx).Infof("开始处理用户注册逻辑，用户名: %s", u.Username)
	if !errors.Is(global.GVA_DB.Where("username = ?", u.Username).First(&user).Error, gorm.ErrRecordNotFound) { // 判断用户名是否注册
		global.LogCtx(ctx).Errorw("检索数据库用户失败（用户名已注册）", "err", err)
		return userInter, errors.New("用户名已注册")
	}
	if !errors.Is(global.GVA_DB.Where("email = ?", u.Email).First(&user).Error, gorm.ErrRecordNotFound) {
		global.LogCtx(ctx).Errorw("检索数据库用户失败（邮箱重复）", "err", err)
		return userInter, errors.New("邮箱重复")
	}
	if !errors.Is(global.GVA_DB.Where("telephone = ?", u.Telephone).First(&user).Error, gorm.ErrRecordNotFound) {
		global.LogCtx(ctx).Errorw("检索数据库用户失败（电话号码重复）", "err", err)
		return userInter, errors.New("电话号码重复")
	}
	u.Password = utils.BcryptHash(u.Password)
	u.UUID = uuid.Must(uuid.New(), nil)
	u.Status = 1
	err = global.GVA_DB.Create(&u).Error
	global.LogCtx(ctx).Info("用户注册成功！")
	return u, err
}

// ResetPassword 重置密码业务
func (s *UserService) ResetPassword(ID uint) (err error) {
	err = global.GVA_DB.Model(&pojo.User{}).Where("id = ?", ID).Update("password", utils.BcryptHash("123456")).Error
	return err
}

// UpLoadHeaderImage 上传头像业务
func (s *UserService) UpLoadHeaderImage(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var u pojo.User
	file, _ := c.FormFile("head-img")
	fileObj, err := file.Open()
	if err != nil {
		fmt.Println(err)
		return
	}
	// 把文件上传到minio对应的桶中
	ok := utils.UpLoadFile("userheaders", file.Filename, fileObj, file.Size)
	if !ok {
		global.SugaredLogger.Error("上传到桶失败")
		return
	}
	headerUrl := utils.GetFileURL("userheaders", file.Filename, time.Second*24*60*60)
	if headerUrl == "" {
		return
	}
	u.HeadImg = headerUrl
	global.GVA_DB.Model(&u).Where("id = ?", id).Update("head_img", headerUrl)
	response.Success(c, headerUrl)
}
