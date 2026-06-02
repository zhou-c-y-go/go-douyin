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
	"github.com/google/uuid"
	"gorm.io/gorm"
	"mime/multipart"
	"strconv"
	"time"
)

type UserService struct {
}

var repository = new(repos.UserRepository)

func (s *UserService) LoginService(ctx context.Context, username, password string) (string, *pojo.User, error) {
	// 1. 查 MySQL 数据库验证身份
	u := &pojo.User{Password: password, Username: username}
	user, err := repository.FindUserByIdentifier(u)
	if err != nil || user == nil {
		return "", nil, errors.New("用户不存在或者密码错误")
	}
	// 2. 状态防御检查
	if user.Status != 1 {
		return "", nil, errors.New("用户被禁止登录")
	}
	// 3. 生产 JWT 令牌
	j := utils.NewJWT()
	claim := j.CreateClaim(request.BaseClaims{
		Id:          user.ID,
		UUID:        user.UUID,
		UserName:    user.Username,
		AuthorityId: user.AuthorityId,
	})
	token, err := j.CreateToken(claim)
	if err != nil {
		global.LogCtx(ctx).Errorw("生成 Token 失败", "err", err)
		return "", nil, errors.New("获取token失败")
	}
	// 4. 将 Token 灌入 Redis 白名单（使用带 TraceID 的 ctx 请求级上下文）
	redisKey := fmt.Sprintf("UserToken:%d", user.ID)
	// 假设你原本的 static.Jwt_time 是 7 天过期，这里用 Go 标准的 time.Duration 承接
	err = global.GVA_REDIS.Set(ctx, redisKey, token, 7*24*time.Hour).Err()
	if err != nil {
		global.LogCtx(ctx).Errorw("💥 致命：Token 写入 Redis 失败！", "err", err)
		return "", nil, errors.New("系统服务异常，请稍后重试")
	}

	global.LogCtx(ctx).Infof("🔑 [Service] 成功为用户 [%s] 签发令牌并同步写入 Redis", user.Username)

	return token, user, nil
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
func (s *UserService) ResetPassword(ctx context.Context, ID uint) (err error) {
	err = global.GVA_DB.Model(&pojo.User{}).Where("id = ?", ID).Update("password", utils.BcryptHash("123456")).Error
	if err != nil {
		global.LogCtx(ctx).Errorw("没有找到该用户", "err", err)
	}
	return err
}

// GetUserProfileService 获取主页用户信息
func (s *UserService) GetUserProfileService(ctx context.Context, userID int64, username string) (*response.UserProfileResponse, error) {
	var resp response.UserProfileResponse
	resp.ID = userID
	resp.Username = username

	redisKey := fmt.Sprintf("UserProfile:%d", userID)

	fields, err := global.GVA_REDIS.HGetAll(ctx, redisKey).Result()
	if err == nil && len(fields) > 0 {
		global.LogCtx(ctx).Infoln("🚀 [Service] 内存击中用户主页 Hash 缓存，高并发直接返回！")
		resp.Avatar = fields["HeadImg"]
		resp.BackgroundImage = fields["BackgroundImage"]
		resp.Signature = fields["Signature"]
		resp.TotalLiked, _ = strconv.ParseInt(fields["TotalFavorited"], 10, 64)
		resp.WorkCount, _ = strconv.ParseInt(fields["WorkCount"], 10, 64)
		resp.FavoriteCount, _ = strconv.ParseInt(fields["FavoriteCount"], 10, 64)
		resp.Gender = fields["Gender"]
		return &resp, nil
	}
	global.LogCtx(ctx).Warnf("⚠️  [Service] 缓存未命中(或已过期), 准备携带 TraceID 回源 MySQL 捞取数据...")
	var user pojo.User
	if err := global.GVA_DB.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, errors.New("用户不存在")
	}
	resp.Avatar = user.HeadImg
	resp.BackgroundImage = user.BackgroundImage
	resp.Signature = user.Signature
	resp.TotalLiked = user.TotalFavorited
	resp.WorkCount = user.WorkCount
	resp.FavoriteCount = user.FavoriteCount
	resp.Gender = user.Gender
	go func(uid int64, u pojo.User, traceCtx context.Context) {
		cacheMap := map[string]interface{}{
			"Username":        u.Username,
			"HeadImg":         u.HeadImg,
			"BackgroundImage": u.BackgroundImage,
			"Signature":       u.Signature,
			"TotalFavorited":  u.TotalFavorited,
			"WorkCount":       u.WorkCount,
			"FavoriteCount":   u.FavoriteCount,
			"Gender":          u.Gender,
		}

		key := fmt.Sprintf("UserProfile:%d", uid)
		// 异步写入使用带 TraceID 的专属后台上下文
		global.GVA_REDIS.HMSet(traceCtx, key, cacheMap)
		global.GVA_REDIS.Expire(traceCtx, key, 7*24*time.Hour) // 假设写死7天，你可以用静态变量
		global.LogCtx(traceCtx).Infof("✅ [Async] 用户 [%d] 缓存异步铺设完毕.", uid)
	}(userID, user, context.WithoutCancel(ctx)) // 👈 完美保护生命周期并继承了 TraceID

	return &resp, nil
}

// UpdateUserInfoService 更新用户个人信息
func (s *UserService) UpdateUserInfoService(ctx context.Context, reqUser *pojo.User) error {
	rows := repository.Update(reqUser)
	if rows == 0 {
		return errors.New("未修改任何信息或更新失败")
	}
	redisKey := fmt.Sprintf("UserProfile:%d", reqUser.ID)
	// 使用 WithoutCancel 剥离生命周期，保留 TraceID
	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, key string, uid int64) {
		// 传递入参进来的安全变量，拒绝高并发闭包逃逸 Bug
		err := global.GVA_REDIS.Del(traceCtx, key).Err()
		if err != nil {
			global.LogCtx(traceCtx).Errorw("清除用户主页缓存失败，可能导致读取到脏数据", "err", err)
		} else {
			global.LogCtx(traceCtx).Infof("🔥 [Async] 用户 [%d] 信息更新成功，已强行淘汰旧Redis 缓存", uid)
		}
	}(detachedCtx, redisKey, reqUser.ID)
	return nil
}

func (s *UserService) UpLoadHeaderImage(ctx context.Context, u *pojo.User, id int, file *multipart.FileHeader, fileObj multipart.File) error {
	// 把文件上传到minio对应的桶中
	ok := utils.UpLoadFile(ctx, "userheaders", file.Filename, fileObj, file.Size)
	if !ok {
		global.LogCtx(ctx).Error("上传到桶失败")
		return errors.New("上传到桶失败")
	}
	headerUrl := utils.GetFileURL(ctx, "userheaders", file.Filename, time.Hour*24*7)
	if headerUrl == "" {
		global.LogCtx(ctx).Error("没有在桶中找到用户头像地址")
		return errors.New("没有在桶中找到用户头像地址")
	}
	u.ID = int64(id)
	u.HeadImg = headerUrl
	if repository.UpdateHeadImag(u, headerUrl) == 0 {
		global.LogCtx(ctx).Error("未修改任何信息或更新失败")
		return errors.New("未修改任何信息或更新失败")
	}
	redisKey := fmt.Sprintf("UserProfile:%d", id)
	detachedCtx := context.WithoutCancel(ctx) // 👈 强行复制克隆 TraceID，剥离死亡连坐

	go func(traceCtx context.Context, key string, url string) {
		// 💡 核心修复：先用 Exists 探探路，看这个用户的整个主页缓存还在不在
		exists, err := global.GVA_REDIS.Exists(traceCtx, key).Result()
		if err != nil {
			global.LogCtx(traceCtx).Errorw("探查 Redis 缓存状态失败", "err", err)
			return
		}
		if exists == 0 {
			// 情况 A：缓存本来就过期死透了，那咱们直接补一刀 Del 清空残余，
			// 下次用户进主页时，会自动安全地整体回源 MySQL 重新铺设完整缓存！
			global.GVA_REDIS.Del(traceCtx, key)
			global.LogCtx(traceCtx).Infof("🔥 [Async] 用户 [%d] 缓存本身不存在，已执行 Del 清空确保一致性", id)
		} else {
			// 情况 B：缓存还在，咱们再放心地精准修改 Hash 里的 HeadImg 字段，绝不破坏 signature 等其他兄弟数据！
			err = global.GVA_REDIS.HSet(traceCtx, key, "HeadImg", url).Err()
			if err != nil {
				global.LogCtx(traceCtx).Errorw("精准更新缓存头像失败", "err", err)
			} else {
				global.LogCtx(traceCtx).Infof("⚡ [Async] 用户 [%d] 在线更新头像，已成功无感精准刷新缓存 Hash", id)
			}
		}
	}(detachedCtx, redisKey, headerUrl)
	return nil
}
