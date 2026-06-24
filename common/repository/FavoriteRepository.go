package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
	"time"
)

type FavoriteRepository interface {
	IsMemberFavoriteSet(ctx context.Context, targetType string, targetID, userID int64) (bool, error)
	AddFavoriteSetAndIncrCount(ctx context.Context, targetType string, targetID, userID int64) error
	RemFavoriteSetAndDecrCount(ctx context.Context, targetType string, targetID, userID int64) error
	GetFavoriteVideoIDs(ctx context.Context, userID int64) ([]int64, error)

	// 🎯 升级后注入的高并发管道基础设施
	MarkFavoriteTargetAsDirty(ctx context.Context, dirtyToken string) error
	PushFavoriteRecordQueue(ctx context.Context, payload string) error
	StartFavoriteFlushWorker(ctx context.Context) // 👈 属于收藏的无敌永动机
}

type favoriteRepository struct{}

func NewFavoriteRepository() FavoriteRepository { return &favoriteRepository{} } //

func (r *favoriteRepository) IsMemberFavoriteSet(ctx context.Context, targetType string, targetID, userID int64) (bool, error) {
	favoriteSetKey := fmt.Sprintf("Favorite:Set:%s:%d", targetType, targetID) //
	return global.GVA_REDIS.SIsMember(ctx, favoriteSetKey, userID).Result()   //
}

func (r *favoriteRepository) AddFavoriteSetAndIncrCount(ctx context.Context, targetType string, targetID, userID int64) error {
	pipe := global.GVA_REDIS.Pipeline()                                               //
	pipe.SAdd(ctx, fmt.Sprintf("Favorite:Set:%s:%d", targetType, targetID), userID)   //
	pipe.Incr(ctx, fmt.Sprintf("Favorite:Count:%s:%d", targetType, targetID))         //
	pipe.SAdd(ctx, fmt.Sprintf("User:Favorite:%ss:%d", targetType, userID), targetID) //
	_, err := pipe.Exec(ctx)                                                          //
	return err
}

func (r *favoriteRepository) RemFavoriteSetAndDecrCount(ctx context.Context, targetType string, targetID, userID int64) error {
	pipe := global.GVA_REDIS.Pipeline()                                               //
	pipe.SRem(ctx, fmt.Sprintf("Favorite:Set:%s:%d", targetType, targetID), userID)   //
	pipe.Decr(ctx, fmt.Sprintf("Favorite:Count:%s:%d", targetType, targetID))         //
	pipe.SRem(ctx, fmt.Sprintf("User:Favorite:%ss:%d", targetType, userID), targetID) //
	_, err := pipe.Exec(ctx)                                                          //
	return err
}

func (r *favoriteRepository) MarkFavoriteTargetAsDirty(ctx context.Context, dirtyToken string) error {
	return global.GVA_REDIS.SAdd(ctx, "mix:dirty:favorite_targets", dirtyToken).Err()
}

func (r *favoriteRepository) PushFavoriteRecordQueue(ctx context.Context, payload string) error {
	return global.GVA_REDIS.RPush(ctx, "list:dirty:favorite_records", payload).Err()
}

// StartFavoriteFlushWorker ── 🦾 收藏专属永动机：批量吞噬一切收藏计数与历史关系
func (r *favoriteRepository) StartFavoriteFlushWorker(ctx context.Context) {
	defer func() {
		if rec := recover(); rec != nil {
			global.LogCtx(ctx).Errorw("🚨 [Panic-Recovery] 后台收藏永动机瘫痪，正在全自动重启自愈...", "err", rec)
			time.Sleep(time.Second)
			go r.StartFavoriteFlushWorker(context.Background())
		}
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// -----------------------------------------------------------------
			// 🛠️ 核心任务一：【收藏流水关系】批量 Upsert 落盘（每 100 条合并成一发SQL）
			// -----------------------------------------------------------------
			recordLedgerKey := "list:dirty:favorite_records"
			rawRecords, err := global.GVA_REDIS.LRange(ctx, recordLedgerKey, 0, 499).Result()
			if err == nil && len(rawRecords) > 0 {
				var insertRecords []pojo.FavoriteRecord
				for _, raw := range rawRecords {
					parts := strings.Split(raw, ":")
					uID, _ := strconv.ParseInt(parts[0], 10, 64)
					tID, _ := strconv.ParseInt(parts[1], 10, 64)
					tType := parts[2]
					status, _ := strconv.Atoi(parts[3])

					insertRecords = append(insertRecords, pojo.FavoriteRecord{
						UserID: uID, TargetID: tID, TargetType: tType, Status: int8(status),
					})
				}

				if len(insertRecords) > 0 {
					// 一枪批量插入，且利用联合唯一索引解决冲突更新
					global.GVA_DB.WithContext(ctx).Clauses(clause.OnConflict{
						Columns:   []clause.Column{{Name: "user_id"}, {Name: "target_id"}, {Name: "target_type"}},
						DoUpdates: clause.Assignments(map[string]interface{}{"status": gorm.Expr("VALUES(status)")}),
					}).CreateInBatches(&insertRecords, 100)

					// 裁剪队列
					global.GVA_REDIS.LTrim(ctx, recordLedgerKey, int64(len(rawRecords)), -1)
				}
			}

			// -----------------------------------------------------------------
			// 🛠️ 核心任务二：【收藏总数 Count】批量合并削峰 UPDATE
			// -----------------------------------------------------------------
			dirtyLedgerKey := "mix:dirty:favorite_targets"
			dirtyTokens, _ := global.GVA_REDIS.SPopN(ctx, dirtyLedgerKey, 1000).Result()
			if len(dirtyTokens) > 0 {
				pipe := global.GVA_REDIS.Pipeline()
				cmds := make([]*redis.StringCmd, len(dirtyTokens))
				for i, token := range dirtyTokens {
					cmds[i] = pipe.Get(ctx, fmt.Sprintf("Favorite:Count:%s", token))
				}
				_, _ = pipe.Exec(ctx)

				tx := global.GVA_DB.Begin()
				for i, token := range dirtyTokens {
					currentCount, cErr := cmds[i].Int64()
					if cErr != nil {
						continue
					}
					parts := strings.Split(token, ":")
					if len(parts) != 2 {
						continue
					}
					targetType := parts[0]
					targetID, _ := strconv.ParseInt(parts[1], 10, 64)

					if targetType == "video" {
						tx.Model(&pojo.Video{}).Where("id = ?", targetID).Update("favorite_count", currentCount)
					} else if targetType == "article" {
						tx.Model(&pojo.Article{}).Where("id = ?", targetID).Update("favorite_count", currentCount)
					}
				}
				tx.Commit()
				global.LogCtx(ctx).Infof("🚀 [Favor-Write-Back-Engine] 成功合并同步 %d 个热点收藏数字", len(dirtyTokens))
			}
		}
	}
}

// GetFavoriteVideoIDs ── 保持你原有的懒加载回源...
func (r *favoriteRepository) GetFavoriteVideoIDs(ctx context.Context, userID int64) ([]int64, error) {
	userFavoriteKey := fmt.Sprintf("User:Favorite:Videos:%d", userID)     //
	exists, err := global.GVA_REDIS.Exists(ctx, userFavoriteKey).Result() //
	if err == nil && exists > 0 {                                         //
		members, err := global.GVA_REDIS.SMembers(ctx, userFavoriteKey).Result() //
		if err == nil && len(members) > 0 {                                      //
			ids := make([]int64, 0, len(members)) //
			for _, m := range members {           //
				if id, parseErr := strconv.ParseInt(m, 10, 64); parseErr == nil { //
					ids = append(ids, id) //
				}
			}
			return ids, nil //
		}
	}
	var ids []int64                                                     //
	err = global.GVA_DB.WithContext(ctx).Model(&pojo.FavoriteRecord{}). //
										Where("user_id = ? AND target_type = ? AND status = 1", userID, "video"). //
										Order("updated_at DESC").                                                 //
										Pluck("target_id", &ids).Error                                            //

	if err == nil && len(ids) > 0 { //
		detachedCtx := context.WithoutCancel(ctx)                   //
		go func(detachedCtx context.Context, FavoriteIDs []int64) { //
			members := make([]interface{}, len(FavoriteIDs)) //
			for i, id := range FavoriteIDs {                 //
				members[i] = id //
			}
			pipe := global.GVA_REDIS.Pipeline()                     //
			pipe.SAdd(detachedCtx, userFavoriteKey, members...)     //
			pipe.Expire(detachedCtx, userFavoriteKey, 24*time.Hour) //
			_, _ = pipe.Exec(detachedCtx)                           //
		}(detachedCtx, ids) //
	}
	return ids, err //
}
