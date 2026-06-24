package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/utils"
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

// UserRepository 用户领域持久化标准接口
type UserRepository interface {
	QueryByID(ctx context.Context, id int) (pojo.User, error)
	FindUserByIdentifier(ctx context.Context, u *pojo.User) (*pojo.User, error)
	Update(ctx context.Context, user *pojo.User) (int64, error)
	UpdateHeadImg(ctx context.Context, u *pojo.User, headerUrl string) (int64, error)
	FindByUsername(ctx context.Context, username string) (*pojo.User, error)
	FindByEmail(ctx context.Context, email string) (*pojo.User, error)
	FindByTelephone(ctx context.Context, telephone string) (*pojo.User, error)
	Create(ctx context.Context, u *pojo.User) error
	UpdatePassword(ctx context.Context, id uint, hashedPassword string) error

	// Cache 缓存自治防线
	SetTokenCache(ctx context.Context, userID int64, token string, ttl time.Duration) error
	GetProfileCache(ctx context.Context, userID int64) (map[string]string, error)
	SetProfileCache(ctx context.Context, userID int64, cacheMap map[string]interface{}, ttl time.Duration) error
	DelProfileCache(ctx context.Context, userID int64) error
	ExistsProfileCache(ctx context.Context, userID int64) (int64, error)
	HSetProfileCache(ctx context.Context, userID int64, field string, value interface{}) error
	BatchGetUserCardMap(ctx context.Context, userIDs []int64) (map[int64]response.UserCardInfo, error)
}

type userRepository struct{}

func NewUserRepository() UserRepository {
	return &userRepository{}
}

func (r *userRepository) QueryByID(ctx context.Context, id int) (pojo.User, error) {
	var user pojo.User
	err := global.GVA_DB.WithContext(ctx).Find(&user, id).Error
	if err != nil || user.ID == 0 {
		return pojo.User{}, errors.New("用户不存在")
	}
	return user, nil
}

func (r *userRepository) FindUserByIdentifier(ctx context.Context, u *pojo.User) (*pojo.User, error) {
	var user pojo.User
	err := global.GVA_DB.WithContext(ctx).Where("username = ? OR telephone = ? OR email = ?", u.Username, u.Username, u.Username).First(&user).Error
	if err != nil || user.ID == 0 {
		global.SugaredLogger.Errorf("query error: username is %s", u.Username)
		return nil, err
	}
	if ok := utils.BcryptCheck(u.Password, user.Password); !ok {
		return nil, errors.New("密码错误")
	}
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, user *pojo.User) (int64, error) {
	tx := global.GVA_DB.WithContext(ctx).Model(&pojo.User{}).Where("id = ?", user.ID).Updates(user)
	return tx.RowsAffected, tx.Error
}

func (r *userRepository) UpdateHeadImg(ctx context.Context, u *pojo.User, headerUrl string) (int64, error) {
	tx := global.GVA_DB.WithContext(ctx).Model(u).Where("id = ?", u.ID).Update("head_img", headerUrl)
	return tx.RowsAffected, tx.Error
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*pojo.User, error) {
	var user pojo.User
	err := global.GVA_DB.WithContext(ctx).Where("username = ?", username).First(&user).Error
	return &user, err
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*pojo.User, error) {
	var user pojo.User
	err := global.GVA_DB.WithContext(ctx).Where("email = ?", email).First(&user).Error
	return &user, err
}

func (r *userRepository) FindByTelephone(ctx context.Context, telephone string) (*pojo.User, error) {
	var user pojo.User
	err := global.GVA_DB.WithContext(ctx).Where("telephone = ?", telephone).First(&user).Error
	return &user, err
}

func (r *userRepository) Create(ctx context.Context, u *pojo.User) error {
	return global.GVA_DB.WithContext(ctx).Create(u).Error
}

func (r *userRepository) UpdatePassword(ctx context.Context, id uint, hashedPassword string) error {
	return global.GVA_DB.WithContext(ctx).Model(&pojo.User{}).Where("id = ?", id).Update("password", hashedPassword).Error
}

func (r *userRepository) SetTokenCache(ctx context.Context, userID int64, token string, ttl time.Duration) error {
	redisKey := fmt.Sprintf("UserToken:%d", userID)
	return global.GVA_REDIS.Set(ctx, redisKey, token, ttl).Err()
}

func (r *userRepository) GetProfileCache(ctx context.Context, userID int64) (map[string]string, error) {
	redisKey := fmt.Sprintf("UserProfile:%d", userID)
	return global.GVA_REDIS.HGetAll(ctx, redisKey).Result()
}

func (r *userRepository) SetProfileCache(ctx context.Context, userID int64, cacheMap map[string]interface{}, ttl time.Duration) error {
	redisKey := fmt.Sprintf("UserProfile:%d", userID)
	if err := global.GVA_REDIS.HSet(ctx, redisKey, cacheMap).Err(); err != nil {
		return err
	}
	return global.GVA_REDIS.Expire(ctx, redisKey, ttl).Err()
}

func (r *userRepository) DelProfileCache(ctx context.Context, userID int64) error {
	redisKey := fmt.Sprintf("UserProfile:%d", userID)
	return global.GVA_REDIS.Del(ctx, redisKey).Err()
}

func (r *userRepository) ExistsProfileCache(ctx context.Context, userID int64) (int64, error) {
	redisKey := fmt.Sprintf("UserProfile:%d", userID)
	return global.GVA_REDIS.Exists(ctx, redisKey).Result()
}

func (r *userRepository) HSetProfileCache(ctx context.Context, userID int64, field string, value interface{}) error {
	redisKey := fmt.Sprintf("UserProfile:%d", userID)
	return global.GVA_REDIS.HSet(ctx, redisKey, field, value).Err()
}

func (r *userRepository) BatchGetUserCardMap(ctx context.Context, userIDs []int64) (map[int64]response.UserCardInfo, error) {
	userCardMap := make(map[int64]response.UserCardInfo)
	if len(userIDs) == 0 {
		return userCardMap, nil
	}
	// 1. Pipeline 批量读取
	pipe := global.GVA_REDIS.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(userIDs))
	for i, uid := range userIDs {
		cmds[i] = pipe.HGetAll(ctx, fmt.Sprintf("UserProfile:%d", uid))
	}
	_, _ = pipe.Exec(ctx)
	missedUIDs := make([]int64, 0)
	for i, uid := range userIDs {
		resMap, err := cmds[i].Result()
		// 🎯 终极刺客杀手：用 ID 作为探测雷达！因为数据库里 ID 绝对不可能为空！
		idStr := resMap["ID"]
		if idStr == "" {
			idStr = resMap["id"]
		}
		if err == nil && idStr != "" {
			// 缓存绝对击中！哪怕他没名字、没头像，只要有 ID 就算命中！
			username := resMap["Username"]
			if username == "" {
				username = resMap["username"]
			}

			avatar := resMap["HeadImg"]
			if avatar == "" {
				avatar = resMap["headImg"]
			}
			userCardMap[uid] = response.UserCardInfo{
				ID:       uid,
				Username: username,
				Avatar:   avatar,
			}
		} else {
			missedUIDs = append(missedUIDs, uid)
		}
	}
	// 2. 补票收网
	if len(missedUIDs) > 0 {
		var pojoUsers []pojo.User
		err := global.GVA_DB.WithContext(ctx).Where("id IN ?", missedUIDs).Find(&pojoUsers).Error
		if err == nil && len(pojoUsers) > 0 {
			for _, u := range pojoUsers {
				userCardMap[u.ID] = response.UserCardInfo{ID: u.ID, Username: u.Username, Avatar: u.HeadImg}
			}
			// 3. 异步回填
			detachedCtx := context.WithoutCancel(ctx)
			go func(bgCtx context.Context, dbUsers []pojo.User) {
				defer func() {
					if r1 := recover(); r1 != nil {
						global.LogCtx(bgCtx).Errorw("🚨 缓存回填协程崩溃", "err", r1)
					}
				}()
				writePipe := global.GVA_REDIS.Pipeline()
				for _, u := range dbUsers {
					redisKey := fmt.Sprintf("UserProfile:%d", u.ID)

					// 💡 强转为 String，使用更安全的 HSet 替代 HMSet，杜绝一切静默写入失败！
					idStr := strconv.FormatInt(u.ID, 10)
					profileMap := map[string]interface{}{
						"ID":       idStr,
						"id":       idStr,
						"Username": u.Username,
						"username": u.Username,
						"HeadImg":  u.HeadImg,
						"headImg":  u.HeadImg,
					}

					writePipe.HSet(bgCtx, redisKey, profileMap)
					writePipe.Expire(bgCtx, redisKey, 24*time.Hour)
				}
				// 💡 捕捉并暴露底层写入错误！
				_, execErr := writePipe.Exec(bgCtx)
				if execErr != nil {
					global.LogCtx(bgCtx).Errorw("❌ [Cache-Backfill] 严重：Redis底层拒绝写入！", "error", execErr)
				} else {
					global.LogCtx(bgCtx).Infof("🚀 [Cache-Backfill] 成功为 %d 名用户完成回填", len(dbUsers))
				}
			}(detachedCtx, pojoUsers)
		}
	}
	return userCardMap, nil
}
