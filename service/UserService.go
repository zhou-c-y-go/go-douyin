package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/model/request"
	"Go_Project/common/model/response"
	repo "Go_Project/common/repository"
	"Go_Project/utils"
	"context"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"mime/multipart"
	"strconv"
	"time"
)

// UserService 用户模块业务标杆接口
type UserService interface {
	LoginService(ctx context.Context, username, password string) (string, *pojo.User, error)
	Register(ctx context.Context, u pojo.User) (pojo.User, error)
	ResetPassword(ctx context.Context, ID uint) error
	GetUserProfileService(ctx context.Context, userID int64, username string) (*response.UserProfileResponse, error)
	UpdateUserInfoService(ctx context.Context, reqUser *pojo.User) error
	UpLoadHeaderImage(ctx context.Context, u *pojo.User, id int, file *multipart.FileHeader, fileObj multipart.File) error
}

type userService struct {
	userRepo repo.UserRepository
}

func NewUserService(ur repo.UserRepository) UserService {
	return &userService{userRepo: ur}
}

func (s *userService) LoginService(ctx context.Context, username, password string) (string, *pojo.User, error) {
	u := &pojo.User{Password: password, Username: username}
	user, err := s.userRepo.FindUserByIdentifier(ctx, u)
	if err != nil || user == nil {
		return "", nil, errors.New("用户不存在或者密码错误")
	}
	if user.Status != 1 {
		return "", nil, errors.New("用户被禁止登录")
	}

	j := utils.NewJWT()
	claim := j.CreateClaim(request.BaseClaims{
		Id:          user.ID,
		UUID:        user.UUID,
		UserName:    user.Username,
		AuthorityId: user.AuthorityId,
	})
	token, err := j.CreateToken(claim)
	if err != nil {
		return "", nil, errors.New("获取token失败")
	}

	// 托付给持久层管理缓存寿命
	if err = s.userRepo.SetTokenCache(ctx, user.ID, token, 7*24*time.Hour); err != nil {
		return "", nil, errors.New("系统服务异常，请稍后重试")
	}
	return token, user, nil
}

func (s *userService) Register(ctx context.Context, u pojo.User) (pojo.User, error) {
	if _, err := s.userRepo.FindByUsername(ctx, u.Username); !errors.Is(err, gorm.ErrRecordNotFound) {
		return pojo.User{}, errors.New("用户名已注册")
	}
	if _, err := s.userRepo.FindByEmail(ctx, u.Email); !errors.Is(err, gorm.ErrRecordNotFound) {
		return pojo.User{}, errors.New("邮箱重复")
	}
	if _, err := s.userRepo.FindByTelephone(ctx, u.Telephone); !errors.Is(err, gorm.ErrRecordNotFound) {
		return pojo.User{}, errors.New("电话号码重复")
	}

	u.Password = utils.BcryptHash(u.Password)
	u.UUID = uuid.Must(uuid.New(), nil)
	u.Status = 1
	err := s.userRepo.Create(ctx, &u)
	return u, err
}

func (s *userService) ResetPassword(ctx context.Context, ID uint) error {
	hashedPwd := utils.BcryptHash("123456")
	return s.userRepo.UpdatePassword(ctx, ID, hashedPwd)
}

func (s *userService) GetUserProfileService(ctx context.Context, userID int64, username string) (*response.UserProfileResponse, error) {
	resp := &response.UserProfileResponse{ID: userID, Username: username}

	fields, err := s.userRepo.GetProfileCache(ctx, userID)
	if err == nil && len(fields) > 0 {
		resp.Avatar = fields["HeadImg"]
		resp.BackgroundImage = fields["BackgroundImage"]
		resp.Signature = fields["Signature"]
		resp.TotalLiked, _ = strconv.ParseInt(fields["TotalFavorited"], 10, 64)
		resp.WorkCount, _ = strconv.ParseInt(fields["WorkCount"], 10, 64)
		resp.FavoriteCount, _ = strconv.ParseInt(fields["FavoriteCount"], 10, 64)
		resp.Gender = fields["Gender"]
		return resp, nil
	}

	user, err := s.userRepo.QueryByID(ctx, int(userID))
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	resp.Avatar = user.HeadImg
	resp.BackgroundImage = user.BackgroundImage
	resp.Signature = user.Signature
	resp.Gender = user.Gender

	// 异步回填
	go func(uid int64, u pojo.User, traceCtx context.Context) {
		cacheMap := map[string]interface{}{
			"Username":        u.Username,
			"HeadImg":         u.HeadImg,
			"BackgroundImage": u.BackgroundImage,
			"Signature":       u.Signature,
			"Gender":          u.Gender,
			"ID":              u.ID,
		}
		_ = s.userRepo.SetProfileCache(traceCtx, uid, cacheMap, 7*24*time.Hour)
	}(userID, user, context.WithoutCancel(ctx))

	return resp, nil
}

func (s *userService) UpdateUserInfoService(ctx context.Context, reqUser *pojo.User) error {
	rows, err := s.userRepo.Update(ctx, reqUser)
	if err != nil || rows == 0 {
		return errors.New("未修改任何信息或更新失败")
	}

	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, uid int64) {
		_ = s.userRepo.DelProfileCache(traceCtx, uid)
	}(detachedCtx, reqUser.ID)
	return nil
}

func (s *userService) UpLoadHeaderImage(ctx context.Context, u *pojo.User, id int, file *multipart.FileHeader, fileObj multipart.File) error {
	if !utils.UpLoadFile(ctx, "userheaders", file.Filename, fileObj, file.Size) {
		return errors.New("上传到桶失败")
	}
	headerUrl := utils.GetFileURL(ctx, "userheaders", file.Filename, time.Hour*24*7)
	if headerUrl == "" {
		return errors.New("没有在桶中找到用户头像地址")
	}

	u.ID = int64(id)
	u.HeadImg = headerUrl
	rows, err := s.userRepo.UpdateHeadImg(ctx, u, headerUrl)
	if err != nil || rows == 0 {
		return errors.New("未修改任何信息或更新失败")
	}

	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, uid int64, url string) {
		exists, err := s.userRepo.ExistsProfileCache(traceCtx, uid)
		if err != nil {
			return
		}
		if exists == 0 {
			_ = s.userRepo.DelProfileCache(traceCtx, uid)
		} else {
			_ = s.userRepo.HSetProfileCache(traceCtx, uid, "HeadImg", url)
		}
	}(detachedCtx, int64(id), headerUrl)
	return nil
}
