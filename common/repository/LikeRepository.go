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

type LikeRepository interface {
	// IsMemberLikeSet Redis 基础原子交互
	IsMemberLikeSet(ctx context.Context, targetType string, targetID, userID int64) (bool, error)
	AddLikeSetAndIncrCount(ctx context.Context, targetType string, targetID, userID int64) error
	RemLikeSetAndDecrCount(ctx context.Context, targetType string, targetID, userID int64) error
	// BatchGetLikeSetCounts Pipeline 批量用户点赞大账本计数
	BatchGetLikeSetCounts(ctx context.Context, userLikeSetKeys []string) ([]int64, error)
	// SyncLikeRecordToDB MySQL 事务异步双写与原子累加
	SyncLikeRecordToDB(ctx context.Context, userID, targetID int64, targetType string, status int, delta int64) error

	// 分布式锁与分批校准基础设施
	SetNXLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	DelLock(ctx context.Context, key string) error
	GetVideoIDsCursor(ctx context.Context, lastID int64, batchSize int) ([]int64, error)
	GetRealLikeCountsFromDB(ctx context.Context, videoIDs []int64) (map[int64]int64, error)
	UpdateVideoLikeCount(ctx context.Context, videoID int64, count int64) error
	SetVideoLikeCountCache(ctx context.Context, videoID int64, count int64, ttl time.Duration) error
}

type likeRepository struct{}

func NewLikeRepository() LikeRepository { return &likeRepository{} }

func (r *likeRepository) IsMemberLikeSet(ctx context.Context, targetType string, targetID, userID int64) (bool, error) {
	likeSetKey := fmt.Sprintf("Like:Set:%s:%d", targetType, targetID)
	return global.GVA_REDIS.SIsMember(ctx, likeSetKey, userID).Result()
}

func (r *likeRepository) AddLikeSetAndIncrCount(ctx context.Context, targetType string, targetID, userID int64) error {
	pipe := global.GVA_REDIS.Pipeline()
	pipe.SAdd(ctx, fmt.Sprintf("Like:Set:%s:%d", targetType, targetID), userID)
	pipe.Incr(ctx, fmt.Sprintf("Like:Count:%s:%d", targetType, targetID))
	pipe.SAdd(ctx, fmt.Sprintf("User:Like:%ss:%d", targetType, userID), targetID) // 联动观众红心全景图
	_, err := pipe.Exec(ctx)
	return err
}

func (r *likeRepository) RemLikeSetAndDecrCount(ctx context.Context, targetType string, targetID, userID int64) error {
	pipe := global.GVA_REDIS.Pipeline()
	pipe.SRem(ctx, fmt.Sprintf("Like:Set:%s:%d", targetType, targetID), userID)
	pipe.Decr(ctx, fmt.Sprintf("Like:Count:%s:%d", targetType, targetID))
	pipe.SRem(ctx, fmt.Sprintf("User:Like:%ss:%d", targetType, userID), targetID)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *likeRepository) BatchGetLikeSetCounts(ctx context.Context, userLikeSetKeys []string) ([]int64, error) {
	pipe := global.GVA_REDIS.Pipeline()
	cmds := make([]*redis.IntCmd, 0, len(userLikeSetKeys))
	for _, key := range userLikeSetKeys {
		cmds = append(cmds, pipe.SCard(ctx, key))
	}
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	counts := make([]int64, len(cmds))
	for i, cmd := range cmds {
		counts[i], _ = cmd.Result()
	}
	return counts, nil
}

func (r *likeRepository) SyncLikeRecordToDB(ctx context.Context, userID, targetID int64, targetType string, status int, delta int64) error {
	return global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record pojo.LikeRecord
		if err := tx.Where("user_id = ? AND target_id = ? AND target_type = ?", userID, targetID, targetType).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newRecord := pojo.LikeRecord{UserID: userID, TargetID: targetID, TargetType: targetType, Status: int8(status)}
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
			return tx.Model(&pojo.Video{}).Where("id = ?", targetID).UpdateColumn("like_count", gorm.Expr("like_count + ?", delta)).Error
		} else if targetType == "comment" {
			return tx.Model(&pojo.Comment{}).Where("id = ?", targetID).UpdateColumn("like_count", gorm.Expr("like_count + ?", delta)).Error
		}
		return errors.New("未知的点赞目标类型")
	})
}

func (r *likeRepository) SetNXLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return global.GVA_REDIS.SetNX(ctx, key, time.Now().Unix(), ttl).Result()
}

func (r *likeRepository) DelLock(ctx context.Context, key string) error {
	return global.GVA_REDIS.Del(ctx, key).Err()
}

func (r *likeRepository) GetVideoIDsCursor(ctx context.Context, lastID int64, batchSize int) ([]int64, error) {
	var videoIDs []int64
	err := global.GVA_DB.WithContext(ctx).Model(&pojo.Video{}).Where("id > ?", lastID).Order("id ASC").Limit(batchSize).Pluck("id", &videoIDs).Error
	return videoIDs, err
}

func (r *likeRepository) GetRealLikeCountsFromDB(ctx context.Context, videoIDs []int64) (map[int64]int64, error) {
	var results []struct {
		TargetID int64
		Count    int64
	}
	err := global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).Select("target_id, COUNT(*) as count").Where("target_type = ? AND status = 1 AND target_id IN ?", "video", videoIDs).Group("target_id").Scan(&results).Error
	if err != nil {
		return nil, err
	}
	countMap := make(map[int64]int64)
	for _, res := range results {
		countMap[res.TargetID] = res.Count
	}
	return countMap, nil
}

func (r *likeRepository) UpdateVideoLikeCount(ctx context.Context, videoID int64, count int64) error {
	return global.GVA_DB.WithContext(ctx).Model(&pojo.Video{}).Where("id = ?", videoID).Update("like_count", count).Error
}

func (r *likeRepository) SetVideoLikeCountCache(ctx context.Context, videoID int64, count int64, ttl time.Duration) error {
	return global.GVA_REDIS.Set(ctx, fmt.Sprintf("Like:Count:video:%d", videoID), count, ttl).Err()
}
