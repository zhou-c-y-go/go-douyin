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

type LikeService struct{}

// ToggleLikeService ── 纯代数降维幂等引擎（已完美联动评论全景红心缓存宇宙）
func (s *LikeService) ToggleLikeService(
	ctx context.Context,
	userID int64,
	targetID int64,
	targetType string,
	reqStatus int) (bool, error) {
	likeSetKey := fmt.Sprintf("Like:Set:%s:%d", targetType, targetID)
	likeCountKey := fmt.Sprintf("Like:Count:%s:%d", targetType, targetID)

	// 1. 检查 Redis 中当前用户的点赞状态
	isLiked, err := global.GVA_REDIS.SIsMember(ctx, likeSetKey, userID).Result()
	if err != nil {
		global.LogCtx(ctx).Errorw("在redis中未找到用户的点赞状态，将去mysql中寻找", "error:", err)
		return false, err
	}

	// 计算代数差：reqStatus == 1 代表想点赞，reqStatus == 0 代表想取消
	var delta int64 = 0
	if reqStatus == 1 && !isLiked {
		delta = 1
	} else if reqStatus == 0 && isLiked {
		delta = -1
	}

	// 2. 拦截幂等：如果算出来 delta == 0，直接原地返回
	if delta == 0 {
		return reqStatus == 1, nil
	}

	// 3. 🎯 核心微操：直接轰炸 Redis 内存，瞬间完成状态变更与【评论全景图并网】
	if delta == 1 {
		global.GVA_REDIS.SAdd(ctx, likeSetKey, userID)
		global.GVA_REDIS.Incr(ctx, likeCountKey)

		// 🚀 核心并网：如果是给评论点赞，必须同步追加到该观众的【全量红心缓存】里！
		if targetType == "comment" {
			userLikeKey := fmt.Sprintf("User:Like:Comments:%d", userID)
			global.GVA_REDIS.SAdd(ctx, userLikeKey, targetID)
		} else if targetType == "video" {
			userLikeKey := fmt.Sprintf("User:Like:Videos:%d", userID)
			global.GVA_REDIS.SAdd(ctx, userLikeKey, targetID)
		}
		global.LogCtx(ctx).Infof("[redis] 用户[%d]点赞成功 | Target: %s, ID: %d", userID, targetType, targetID)
	} else {
		global.GVA_REDIS.SRem(ctx, likeSetKey, userID)
		global.GVA_REDIS.Decr(ctx, likeCountKey)
		// 🚀 核心并网：如果是取消评论点赞，必须同步将红心从该观众的【全量红心缓存】中剔除！
		if targetType == "comment" {
			userLikeKey := fmt.Sprintf("User:Like:Comments:%d", userID)
			global.GVA_REDIS.SRem(ctx, userLikeKey, targetID)
		} else if targetType == "video" {
			userLikeKey := fmt.Sprintf("User:Like:Videos:%d", userID)
			global.GVA_REDIS.SRem(ctx, userLikeKey, targetID)
		}
		global.LogCtx(ctx).Infof("[redis] 用户[%d]取消点赞 | Target: %s, ID: %d", userID, targetType, targetID)
	}

	// 4. 🚀 赛博分流：开启异步协程，让 MySQL 在后台慢慢写，绝不拖累前端响应！
	go func(asyncCtx context.Context, uID, tID int64, tType string, status int, d int64) {
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		txErr := global.GVA_DB.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
			var record pojo.LikeRecord

			if findErr := tx.Where("user_id = ? AND target_id = ? AND target_type = ?", uID, tID, tType).First(&record).Error; findErr != nil {
				if errors.Is(findErr, gorm.ErrRecordNotFound) {
					newRecord := pojo.LikeRecord{
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
					UpdateColumn("like_count", gorm.Expr("like_count + ?", d)).Error; countErr != nil {
					return countErr
				}
			} else if tType == "comment" {
				if countErr := tx.Model(&pojo.Comment{}).Where("id = ?", tID).
					UpdateColumn("like_count", gorm.Expr("like_count + ?", d)).Error; countErr != nil {
					return countErr
				}
			} else {
				return errors.New("未知的点赞目标类型")
			}
			return nil
		})

		if txErr != nil {
			global.LogCtx(asyncCtx).Errorw("❌ [Async-DB] 异步落盘 MySQL 遭遇颠覆性瘫痪", "err", txErr, "userID", uID, "targetID", tID)
		}
	}(ctx, userID, targetID, targetType, reqStatus, delta)

	return reqStatus == 1, nil
}

func (s *LikeService) GetUserTotalLikeCount(
	ctx context.Context,
	userID int64) (int64, error) {

	Keys := static.GetLikeList()
	pipe := global.GVA_REDIS.Pipeline()
	cmds := make([]*redis.IntCmd, 0, len(Keys))
	for _, tType := range Keys {
		tTypeNew := utils.GetLikeSetKey(tType, userID)
		key := tTypeNew
		// 例如 video: User:Like:Videos:123, article: User:Like:Articles:123
		cmds = append(cmds, pipe.SCard(ctx, key))
	}
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		global.LogCtx(ctx).Errorw("获取用户点赞总数失败，Pipeline错误", "err", err, "userID", userID)
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
		global.LogCtx(ctx).Infof("用户[%d]的点赞缓存缺失，触发 MySQL 回源", userID)
		type CountResult struct {
			TargetType string
			Count      int64
		}
		var results []CountResult
		err = global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
			Select("target_type, COUNT(*) as count").
			Where("user_id = ? AND target_type IN (?) AND status = 1", userID, Keys).
			Group("target_type").
			Scan(&results).Error
		if err != nil {
			global.LogCtx(ctx).Errorw("MySQL 统计点赞数量失败", "err", err, "userID", userID)
			return 0, err
		}

		// 构建 map 方便查找
		countMap := make(map[string]int64)
		for _, r := range results {
			countMap[r.TargetType] = r.Count
		}

		// 3.2 回填 Redis（使用 Pipeline 批量 SAdd 用户点赞集合）
		pipe = global.GVA_REDIS.Pipeline()
		for _, tType := range Keys {
			key := utils.GetLikeSetKey(tType, userID)
			// 需要获取该用户点赞的所有具体 target_id，以重建集合
			var targetIDs []int64
			dbErr := global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
				Where("user_id = ? AND target_type = ? AND status = 1", userID, tType).
				Pluck("target_id", &targetIDs).Error
			if dbErr != nil {
				global.LogCtx(ctx).Errorw("查询用户点赞 ID 列表失败", "type", tType, "err", dbErr)
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
			global.LogCtx(ctx).Errorw("回填 Redis 用户点赞集合失败", "err", err, "userID", userID)
		}

		// 3.3 重新计算总数
		total = 0
		for _, tType := range Keys {
			total += countMap[tType]
		}
	}
	global.LogCtx(ctx).Infof("用户[%d]总点赞数量（Redis）: %d", userID, total)
	return total, nil
}

const (
	calibrateLockKey = "cron:calibrate:video_counts:lock"
	calibrateLockTTL = 10 * time.Minute
	batchSize        = 1000 // 每批处理的视频数量
)

// CalibrateVideoCounts 校准所有视频的点赞数和收藏数（MySQL + Redis）
// 建议通过 robfig/cron 每日凌晨低峰期调用
func (s *LikeService) CalibrateVideoCounts(ctx context.Context) error {
	// 1. 分布式锁，防止多实例重复执行
	lockOk, err := global.GVA_REDIS.SetNX(ctx, calibrateLockKey, time.Now().Unix(), calibrateLockTTL).Result()
	if err != nil {
		global.LogCtx(ctx).Errorw("获取校准分布式锁失败", "err", err)
		return err
	}
	if !lockOk {
		global.LogCtx(ctx).Warn("已有其他实例在执行校准任务，跳过本次")
		return nil
	}
	defer func() {
		if delErr := global.GVA_REDIS.Del(ctx, calibrateLockKey).Err(); delErr != nil {
			global.LogCtx(ctx).Errorw("释放校准锁失败", "err", delErr)
		}
	}()

	// 2. 分批校准点赞数
	if err := s.calibrateLikeCounts(ctx); err != nil {
		return err
	}

	// 3. 分批校准收藏数
	if err := s.calibrateFavoriteCounts(ctx); err != nil {
		return err
	}

	global.LogCtx(ctx).Info("视频计数校准完成")
	return nil
}

func (s *LikeService) calibrateLikeCounts(ctx context.Context) error {
	var lastID int64 = 0
	for {
		// 分页获取视频 ID
		var videoIDs []int64
		err := global.GVA_DB.WithContext(ctx).Model(&pojo.Video{}).
			Where("id > ?", lastID).
			Order("id ASC").
			Limit(batchSize).
			Pluck("id", &videoIDs).Error
		if err != nil {
			return err
		}
		if len(videoIDs) == 0 {
			break
		}

		// 统计这批视频的真实点赞数
		type CountResult struct {
			TargetID int64
			Count    int64
		}
		var results []CountResult
		err = global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
			Select("target_id, COUNT(*) as count").
			Where("target_type = ? AND status = 1 AND target_id IN ?", "video", videoIDs).
			Group("target_id").
			Scan(&results).Error
		if err != nil {
			return err
		}

		// 构建 map，方便更新
		countMap := make(map[int64]int64, len(results))
		for _, r := range results {
			countMap[r.TargetID] = r.Count
		}

		// 批量更新 MySQL 和 Redis
		for _, vid := range videoIDs {
			realCount := countMap[vid] // 如果没有记录，默认 0
			// 更新 MySQL
			if err := global.GVA_DB.Model(&pojo.Video{}).Where("id = ?", vid).
				Update("like_count", realCount).Error; err != nil {
				global.LogCtx(ctx).Errorw("更新视频点赞数失败", "videoID", vid, "err", err)
				continue
			}
			// 更新 Redis
			likeCountKey := fmt.Sprintf("Like:Count:video:%d", vid)
			if err := global.GVA_REDIS.Set(ctx, likeCountKey, realCount, 24*time.Hour).Err(); err != nil {
				global.LogCtx(ctx).Errorw("设置 Redis 点赞计数失败", "key", likeCountKey, "err", err)
			}
		}

		lastID = videoIDs[len(videoIDs)-1]
	}
	return nil
}

func (s *LikeService) calibrateFavoriteCounts(ctx context.Context) error {
	var lastID int64 = 0
	for {
		var videoIDs []int64
		err := global.GVA_DB.WithContext(ctx).Model(&pojo.Video{}).
			Where("id > ?", lastID).
			Order("id ASC").
			Limit(batchSize).
			Pluck("id", &videoIDs).Error
		if err != nil {
			return err
		}
		if len(videoIDs) == 0 {
			break
		}

		var results []struct {
			TargetID int64
			Count    int64
		}
		err = global.GVA_DB.WithContext(ctx).Model(&pojo.FavoriteRecord{}).
			Select("target_id, COUNT(*) as count").
			Where("target_type = ? AND status = 1 AND target_id IN ?", "video", videoIDs).
			Group("target_id").
			Scan(&results).Error
		if err != nil {
			return err
		}

		countMap := make(map[int64]int64, len(results))
		for _, r := range results {
			countMap[r.TargetID] = r.Count
		}

		for _, vid := range videoIDs {
			realCount := countMap[vid]
			if err := global.GVA_DB.Model(&pojo.Video{}).Where("id = ?", vid).
				Update("favorite_count", realCount).Error; err != nil {
				global.LogCtx(ctx).Errorw("更新视频收藏数失败", "videoID", vid, "err", err)
				continue
			}
			favCountKey := fmt.Sprintf("Favorite:Count:video:%d", vid)
			if err := global.GVA_REDIS.Set(ctx, favCountKey, realCount, 24*time.Hour).Err(); err != nil {
				global.LogCtx(ctx).Errorw("设置 Redis 收藏计数失败", "key", favCountKey, "err", err)
			}
		}

		lastID = videoIDs[len(videoIDs)-1]
	}
	return nil
}
