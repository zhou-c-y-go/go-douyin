package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"Go_Project/static"
	"Go_Project/utils"
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"time"
)

type FavorService struct{}

func (s *FavorService) ToggleFavorService(
	ctx context.Context,
	userID int64,
	targetID int64,
	targetType string,
	reqStatus int) (bool, error) {
	favoriteSetKey := fmt.Sprintf("Favorite:Set:%s:%d", targetType, targetID)
	favoriteCountKey := fmt.Sprintf("Favorite:Count:%s:%d", targetType, targetID)
	isFavor, err := global.GVA_REDIS.SIsMember(ctx, favoriteSetKey, userID).Result()
	if err != nil {
		global.LogCtx(ctx).Errorw("在redis中未找到用户的收藏状态，将去mysql中寻找", "error:", err)
		return false, err
	}
	var delta int64 = 0
	if reqStatus == 1 && !isFavor {
		delta = 1
	} else if reqStatus == 0 && isFavor {
		delta = -1
	}

	// 2. 拦截幂等：如果算出来 delta == 0，直接原地返回
	if delta == 0 {
		return reqStatus == 1, nil
	}
	if delta == 1 {
		global.GVA_REDIS.SAdd(ctx, favoriteSetKey, userID)
		global.GVA_REDIS.Incr(ctx, favoriteCountKey)

		// 🚀 核心并网：如果是给评论收藏，必须同步追加到该观众的【全量红心缓存】里！
		if targetType == "video" {
			userFavorKey := fmt.Sprintf("User:Favorite:Videos:%d", userID)
			global.GVA_REDIS.SAdd(ctx, userFavorKey, targetID)
		} else if targetType == "article" {
			userFavorKey := fmt.Sprintf("User:Favorite:Articles:%d", userID)
			global.GVA_REDIS.SAdd(ctx, userFavorKey, targetID)
		}
		global.LogCtx(ctx).Infof("[redis] 用户收藏[%d]成功 | Target: %s, ID: %d", userID, targetType, targetID)
	} else {
		global.GVA_REDIS.SRem(ctx, favoriteSetKey, userID)
		global.GVA_REDIS.Decr(ctx, favoriteCountKey)

		// 🚀 核心并网：如果是取消评论收藏，必须同步将红心从该观众的【全量红心缓存】中剔除！
		if targetType == "article" {
			userFavoriteKey := fmt.Sprintf("User:Favorite:Articles:%d", userID)
			global.GVA_REDIS.SRem(ctx, userFavoriteKey, targetID)
		} else if targetType == "video" {
			userFavoriteKey := fmt.Sprintf("User:Favorite:Videos:%d", userID)
			global.GVA_REDIS.SRem(ctx, userFavoriteKey, targetID)
		}
		global.LogCtx(ctx).Infof("[redis] 用户[%d]取消收藏 | Target: %s, ID: %d", userID, targetType, targetID)
	}
	go func(asyncCtx context.Context, uID, tID int64, tType string, status int, d int64) {
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		txErr := global.GVA_DB.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
			var record pojo.FavoriteRecord
			if findErr := tx.Where("user_id = ? AND target_id = ? AND target_type = ?", uID, tID, tType).First(&record).Error; findErr != nil {
				if errors.Is(findErr, gorm.ErrRecordNotFound) {
					newRecord := pojo.FavoriteRecord{
						UserID:     uID,
						TargetID:   tID,
						TargetType: tType,
						Status:     int8(status),
					}
					if createErr := tx.Create(&newRecord).Error; createErr != nil {
						return createErr
					}
				} else {
					return findErr
				}
			} else {
				if updateErr := tx.Model(&record).Update("status", status).Error; updateErr != nil {
					return updateErr
				}
			}

			if tType == "video" {
				if countErr := tx.Model(&pojo.Video{}).Where("id = ?", tID).
					UpdateColumn("favorite_count", gorm.Expr("favorite_count + ?", d)).Error; countErr != nil {
					return countErr
				}
			} else if tType == "article" {
				if countErr := tx.Model(&pojo.Article{}).Where("id = ?", tID).
					UpdateColumn("favorite_count", gorm.Expr("favorite_count + ?", d)).Error; countErr != nil {
					return countErr
				}
			} else {
				return errors.New("未知的收藏目标类型")
			}
			return nil
		})

		if txErr != nil {
			global.LogCtx(asyncCtx).Errorw("❌ [Async-DB] 异步落盘 MySQL 遭遇颠覆性瘫痪", "err", txErr, "userID", uID, "targetID", tID)
		}
	}(ctx, userID, targetID, targetType, reqStatus, delta)
	return reqStatus == 1, nil
}

func (s *FavorService) GetFavoriteTotalFavoriteCount(
	ctx context.Context,
	userID int64) (int64, error) {
	Keys := static.GetFavoriteList()
	pipe := global.GVA_REDIS.Pipeline()
	cmds := make([]*redis.IntCmd, 0, len(Keys))
	for _, tType := range Keys {
		tTypeNew := utils.GetFavoriteSetKey(tType, userID)
		key := tTypeNew
		// 例如 video: User:Favorite:Videos:123, article: User:Favorite:Articles:123
		cmds = append(cmds, pipe.SCard(ctx, key))
	}
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		global.LogCtx(ctx).Errorw("获取用户收藏总数失败，Pipeline错误", "err", err, "userID", userID)
		return 0, err
	}
	var total int64 = 0
	allMissing := true
	for _, cmd := range cmds {
		count, err := cmd.Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			global.LogCtx(ctx).Warnw("SCARD命令失败", "err", err)
			continue
		}
		if count > 0 {
			allMissing = false
		}
		total += count
	}
	if allMissing {
		global.LogCtx(ctx).Infof("用户[%d]的收藏缓存缺失，触发 MySQL 回源", userID)
		type CountResult struct {
			TargetType string
			Count      int64
		}
		var results []CountResult
		err = global.GVA_DB.WithContext(ctx).Model(&pojo.FavoriteRecord{}).
			Select("target_type, COUNT(*) as count").
			Where("user_id = ? AND target_type IN (?) AND status = 1", userID, Keys).
			Group("target_type").
			Scan(&results).Error
		if err != nil {
			global.LogCtx(ctx).Errorw("MySQL 统计收藏数量失败", "err", err, "userID", userID)
			return 0, err
		}

		// 构建 map 方便查找
		countMap := make(map[string]int64)
		for _, r := range results {
			countMap[r.TargetType] = r.Count
		}

		// 3.2 回填 Redis（使用 Pipeline 批量 SAdd 用户收藏集合）
		pipe = global.GVA_REDIS.Pipeline()
		for _, tType := range Keys {
			key := utils.GetFavoriteSetKey(tType, userID)
			// 需要获取该用户收藏的所有具体 target_id，以重建集合
			var targetIDs []int64
			dbErr := global.GVA_DB.WithContext(ctx).Model(&pojo.FavoriteRecord{}).
				Where("user_id = ? AND target_type = ? AND status = 1", userID, tType).
				Pluck("target_id", &targetIDs).Error
			if dbErr != nil {
				global.LogCtx(ctx).Errorw("查询用户收藏 ID 列表失败", "type", tType, "err", dbErr)
				continue
			}
			if len(targetIDs) > 0 {
				// 转换为 []interface{}
				members := make([]interface{}, len(targetIDs))
				for i, id := range targetIDs {
					members[i] = id
				}
				pipe.SAdd(ctx, key, members...)
				pipe.Expire(ctx, key, 24*time.Hour)
			}
		}
		if _, err := pipe.Exec(ctx); err != nil {
			global.LogCtx(ctx).Errorw("回填 Redis 用户收藏集合失败", "err", err, "userID", userID)
		}

		// 3.3 重新计算总数
		total = 0
		for _, tType := range Keys {
			total += countMap[tType]
		}
	}
	global.LogCtx(ctx).Infof("用户[%d]总收藏数量（Redis）: %d", userID, total)
	return total, nil
}
