package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"time"
)

type FavoriteRepository interface {
	IsMemberFavoriteSet(ctx context.Context, targetType string, targetID, userID int64) (bool, error)
	AddFavoriteSetAndIncrCount(ctx context.Context, targetType string, targetID, userID int64) error
	RemFavoriteSetAndDecrCount(ctx context.Context, targetType string, targetID, userID int64) error

	BatchGetFavoriteSetCounts(ctx context.Context, userFavSetKeys []string) ([]int64, error)
	GetRealFavoriteCountAndIDs(ctx context.Context, userID int64, targetTypes []string) (map[string]int64, map[string][]int64, error)
	RebuildFavoriteCachePipeline(ctx context.Context, userFavSetKeys map[string]string, targetIDsMap map[string][]int64) error

	SyncFavoriteRecordToDB(ctx context.Context, userID, targetID int64, targetType string, status int, delta int64) error
	GetRealFavoriteCountsFromDB(ctx context.Context, videoIDs []int64) (map[int64]int64, error)
	UpdateVideoFavoriteCount(ctx context.Context, videoID int64, count int64) error
	SetVideoFavoriteCountCache(ctx context.Context, videoID int64, count int64, ttl time.Duration) error
}

type favoriteRepository struct{}

func NewFavoriteRepository() FavoriteRepository { return &favoriteRepository{} }

func (r *favoriteRepository) IsMemberFavoriteSet(ctx context.Context, targetType string, targetID, userID int64) (bool, error) {
	favoriteSetKey := fmt.Sprintf("Favorite:Set:%s:%d", targetType, targetID)
	return global.GVA_REDIS.SIsMember(ctx, favoriteSetKey, userID).Result()
}

func (r *favoriteRepository) AddFavoriteSetAndIncrCount(ctx context.Context, targetType string, targetID, userID int64) error {
	pipe := global.GVA_REDIS.Pipeline()
	pipe.SAdd(ctx, fmt.Sprintf("Favorite:Set:%s:%d", targetType, targetID), userID)
	pipe.Incr(ctx, fmt.Sprintf("Favorite:Count:%s:%d", targetType, targetID))
	pipe.SAdd(ctx, fmt.Sprintf("User:Favorite:%ss:%d", targetType, userID), targetID)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *favoriteRepository) RemFavoriteSetAndDecrCount(ctx context.Context, targetType string, targetID, userID int64) error {
	pipe := global.GVA_REDIS.Pipeline()
	pipe.SRem(ctx, fmt.Sprintf("Favorite:Set:%s:%d", targetType, targetID), userID)
	pipe.Decr(ctx, fmt.Sprintf("Favorite:Count:%s:%d", targetType, targetID))
	pipe.SRem(ctx, fmt.Sprintf("User:Favorite:%ss:%d", targetType, userID), targetID)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *favoriteRepository) BatchGetFavoriteSetCounts(ctx context.Context, userFavSetKeys []string) ([]int64, error) {
	pipe := global.GVA_REDIS.Pipeline()
	cmds := make([]*redis.IntCmd, 0, len(userFavSetKeys))
	for _, key := range userFavSetKeys {
		cmds = append(cmds, pipe.SCard(ctx, key))
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	counts := make([]int64, len(cmds))
	for i, cmd := range cmds {
		counts[i], _ = cmd.Result()
	}
	return counts, nil
}

func (r *favoriteRepository) GetRealFavoriteCountAndIDs(ctx context.Context, userID int64, targetTypes []string) (map[string]int64, map[string][]int64, error) {
	type CountResult struct {
		TargetType string
		Count      int64
	}
	var results []CountResult
	err := global.GVA_DB.WithContext(ctx).Model(&pojo.FavoriteRecord{}).Select("target_type, COUNT(*) as count").Where("user_id = ? AND target_type IN (?) AND status = 1", userID, targetTypes).Group("target_type").Scan(&results).Error
	if err != nil {
		return nil, nil, err
	}

	countMap := make(map[string]int64)
	for _, res := range results {
		countMap[res.TargetType] = res.Count
	}

	targetIDsMap := make(map[string][]int64)
	for _, tType := range targetTypes {
		var ids []int64
		_ = global.GVA_DB.WithContext(ctx).Model(&pojo.FavoriteRecord{}).Where("user_id = ? AND target_type = ? AND status = 1", userID, tType).Pluck("target_id", &ids)
		targetIDsMap[tType] = ids
	}
	return countMap, targetIDsMap, nil
}

func (r *favoriteRepository) RebuildFavoriteCachePipeline(ctx context.Context, userFavSetKeys map[string]string, targetIDsMap map[string][]int64) error {
	pipe := global.GVA_REDIS.Pipeline()
	for tType, key := range userFavSetKeys {
		ids := targetIDsMap[tType]
		if len(ids) > 0 {
			members := make([]interface{}, len(ids))
			for i, v := range ids {
				members[i] = v
			}
			pipe.SAdd(ctx, key, members...)
			pipe.Expire(ctx, key, 24*time.Hour)
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *favoriteRepository) SyncFavoriteRecordToDB(ctx context.Context, userID, targetID int64, targetType string, status int, delta int64) error {
	return global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record pojo.FavoriteRecord
		if err := tx.Where("user_id = ? AND target_id = ? AND target_type = ?", userID, targetID, targetType).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newRecord := pojo.FavoriteRecord{UserID: userID, TargetID: targetID, TargetType: targetType, Status: int8(status)}
				if err := tx.Create(&newRecord).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			if err := tx.Model(&record).Update("status", status).Error; err != nil {
				return err
			}
		}

		if targetType == "video" {
			return tx.Model(&pojo.Video{}).Where("id = ?", targetID).UpdateColumn("favorite_count", gorm.Expr("favorite_count + ?", delta)).Error
		} else if targetType == "article" {
			return tx.Model(&pojo.Article{}).Where("id = ?", targetID).UpdateColumn("favorite_count", gorm.Expr("favorite_count + ?", delta)).Error
		}
		return errors.New("未知的收藏目标类型")
	})
}

func (r *favoriteRepository) GetRealFavoriteCountsFromDB(ctx context.Context, videoIDs []int64) (map[int64]int64, error) {
	var results []struct {
		TargetID int64
		Count    int64
	}
	err := global.GVA_DB.WithContext(ctx).Model(&pojo.FavoriteRecord{}).Select("target_id, COUNT(*) as count").Where("target_type = ? AND status = 1 AND target_id IN ?", "video", videoIDs).Group("target_id").Scan(&results).Error
	if err != nil {
		return nil, err
	}
	countMap := make(map[int64]int64)
	for _, res := range results {
		countMap[res.TargetID] = res.Count
	}
	return countMap, nil
}

func (r *favoriteRepository) UpdateVideoFavoriteCount(ctx context.Context, videoID int64, count int64) error {
	return global.GVA_DB.WithContext(ctx).Model(&pojo.Video{}).Where("id = ?", videoID).Update("favorite_count", count).Error
}

func (r *favoriteRepository) SetVideoFavoriteCountCache(ctx context.Context, videoID int64, count int64, ttl time.Duration) error {
	return global.GVA_REDIS.Set(ctx, fmt.Sprintf("Favorite:Count:video:%d", videoID), count, ttl).Err()
}
