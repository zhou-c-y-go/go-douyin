package service

import (
	"Go_Project/common/model/pojo"
	repo "Go_Project/common/repository"
	"Go_Project/global"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"time"
)

type LikeService struct {
	likeRepo repo.LikeRepository
}

// ToggleLikeService ── 纯代数降维幂等引擎（已完美联动评论全景红心缓存宇宙）
func (s *LikeService) ToggleLikeService(ctx context.Context, userID int64, targetID int64, targetType string, reqStatus int) (bool, error) {
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
		}
		global.LogCtx(ctx).Infof("[redis] 用户[%d]点赞成功 | Target: %s, ID: %d", userID, targetType, targetID)
	} else {
		global.GVA_REDIS.SRem(ctx, likeSetKey, userID)
		global.GVA_REDIS.Decr(ctx, likeCountKey)

		// 🚀 核心并网：如果是取消评论点赞，必须同步将红心从该观众的【全量红心缓存】中剔除！
		if targetType == "comment" {
			userLikeKey := fmt.Sprintf("User:Like:Comments:%d", userID)
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
