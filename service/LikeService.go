package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/repository"
	"Go_Project/global"
	"context"
	"fmt"
	"time"
)

type LikeService interface {
	ToggleLikeService(ctx context.Context, userID, targetID, authorID int64, targetType string, reqStatus int) (bool, error)
	CalibrateVideoCounts(ctx context.Context) error
	GetUserAllCounters(ctx context.Context, userID int64) (map[string]int64, error) // 👈 新增并网大闸
}

type likeService struct {
	likeRepo repository.LikeRepository
}

func NewLikeService(lr repository.LikeRepository) LikeService {
	return &likeService{likeRepo: lr}
}

func (s *likeService) ToggleLikeService(ctx context.Context, userID, targetID, authorID int64, targetType string, reqStatus int) (bool, error) {
	isLiked, err := s.likeRepo.IsMemberLikeSet(ctx, targetType, targetID, userID)
	if err != nil {
		return false, err
	}

	var delta int64 = 0
	if reqStatus == 1 && !isLiked {
		delta = 1
	} else if reqStatus == 0 && isLiked {
		delta = -1
	}

	if delta == 0 {
		return reqStatus == 1, nil
	}

	if delta == 1 {
		_ = s.likeRepo.AddLikeSetAndIncrCount(ctx, targetType, targetID, userID)
	} else {
		_ = s.likeRepo.RemLikeSetAndDecrCount(ctx, targetType, targetID, userID)
	}

	dirtyToken := fmt.Sprintf("%s:%d", targetType, targetID)
	_ = s.likeRepo.MarkTargetAsDirty(ctx, dirtyToken)
	recordPayload := fmt.Sprintf("%d:%d:%s:%d", userID, targetID, targetType, reqStatus)
	_ = s.likeRepo.PushLikeRecordQueue(ctx, recordPayload)

	// =========================================================================
	// 🎯【并网核心】：实时轰炸变更 Redis 中的用户计数器 Hash
	// =========================================================================
	pipe := global.GVA_REDIS.Pipeline()
	// 1. 点赞人（我）给出的点赞总数发生变动
	pipe.HIncrBy(ctx, fmt.Sprintf("User:Counters:%d", userID), "favorite_count", delta)

	// 2. 联动变动创作者受到的总点赞数
	// authorID
	pipe.HIncrBy(ctx, fmt.Sprintf("User:Counters:%d", authorID), "total_like", delta)
	_, _ = pipe.Exec(ctx)

	return reqStatus == 1, nil
}

// GetUserAllCounters ── 🦾 懒加载自愈核心：一枪收网 4 大核心计数器
func (s *likeService) GetUserAllCounters(ctx context.Context, userID int64) (map[string]int64, error) {
	// 1. ⚡ 纯内存拦截
	cachedMap, hit, err := s.likeRepo.GetUserCountersFromCache(ctx, userID)
	if err == nil && hit {
		return cachedMap, nil
	}

	// 2. 🎯 缓存缺失：直接利用你最爱的关系流水表“原地满血复活”
	var favoriteCount int64
	var totalLike int64
	var workCount int64
	var favorCount int64 // 对应你的收藏数

	// A. 查你截图的 like_records 表：数一下操作人点了多少赞
	_ = global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
		Where("user_id = ? AND status = 1", userID).Count(&favoriteCount).Error

	// B. 查视频作品表：数一下这个用户发布了多少个作品
	_ = global.GVA_DB.WithContext(ctx).Model(&pojo.Video{}).
		Where("author_id = ?", userID).Count(&workCount).Error

	// C. 查视频作品表：SUM 聚合求出该用户所有作品累计获得的赞
	_ = global.GVA_DB.WithContext(ctx).Model(&pojo.Video{}).
		Where("author_id = ?", userID).Select("COALESCE(SUM(like_count), 0)").Scan(&totalLike).Error

	// D. 查收藏表：数一下这个用户收藏了多少作品（这里假设你的收藏表叫 favor_records）
	_ = global.GVA_DB.WithContext(ctx).Model(&pojo.FavoriteRecord{}).
		Where("user_id = ? AND status = 1", userID).Count(&favorCount).Error

	// 3. 🦾 组装并回填 Redis 宇宙，死锁防护随后的并发踩踏
	counters := map[string]int64{
		"FavoriteCount": favoriteCount,
		"TotalLike":     totalLike,
		"WorkCount":     workCount,
		"FavorCount":    favorCount,
	}
	_ = s.likeRepo.SetUserCountersCache(ctx, userID, counters, 24*time.Hour)

	return counters, nil
}

// CalibrateVideoCounts ── 保持你原有的校准逻辑...
func (s *likeService) CalibrateVideoCounts(ctx context.Context) error {
	const (
		calibrateLockKey = "cron:calibrate:video_counts:lock"
		calibrateLockTTL = 10 * time.Minute
		batchSize        = 1000
	)
	lockOk, err := s.likeRepo.SetNXLock(ctx, calibrateLockKey, calibrateLockTTL)
	if err != nil || !lockOk {
		return err
	}
	defer func() { _ = s.likeRepo.DelLock(ctx, calibrateLockKey) }()

	var lastID int64 = 0
	for {
		videoIDs, err := s.likeRepo.GetVideoIDsCursor(ctx, lastID, batchSize)
		if err != nil || len(videoIDs) == 0 {
			break
		}
		countMap, _ := s.likeRepo.GetRealLikeCountsFromDB(ctx, videoIDs)
		for _, vid := range videoIDs {
			realCount := countMap[vid]
			_ = s.likeRepo.UpdateVideoLikeCount(ctx, vid, realCount)
			_ = s.likeRepo.SetVideoLikeCountCache(ctx, vid, realCount, 24*time.Hour)
		}
		lastID = videoIDs[len(videoIDs)-1]
	}
	return nil
}
