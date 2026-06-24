package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
	"time"
)

type LikeRepository interface {
	// IsMemberLikeSet Redis 基础原子交互
	IsMemberLikeSet(ctx context.Context, targetType string, targetID, userID int64) (bool, error)
	AddLikeSetAndIncrCount(ctx context.Context, targetType string, targetID, userID int64) error
	RemLikeSetAndDecrCount(ctx context.Context, targetType string, targetID, userID int64) error
	// BatchGetLikeSetCounts Pipeline 批量用户点赞大账本计数
	BatchGetLikeSetCounts(ctx context.Context, userLikeSetKeys []string) ([]int64, error)
	// 分布式锁与分批校准基础设施
	SetNXLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	DelLock(ctx context.Context, key string) error
	GetVideoIDsCursor(ctx context.Context, lastID int64, batchSize int) ([]int64, error)
	GetRealLikeCountsFromDB(ctx context.Context, videoIDs []int64) (map[int64]int64, error)
	UpdateVideoLikeCount(ctx context.Context, videoID int64, count int64) error
	SetVideoLikeCountCache(ctx context.Context, videoID int64, count int64, ttl time.Duration) error
	GetLikedVideoIDs(ctx context.Context, userID int64) ([]int64, error)
	StartLikeFlushWorker(ctx context.Context)
	MarkTargetAsDirty(ctx context.Context, dirtyToken string) error
	PushLikeRecordQueue(ctx context.Context, payload string) error
	GetAuthorID(ctx context.Context, targetType string, targetID int64) (int64, error)
	GetUserCountersFromCache(ctx context.Context, userID int64) (map[string]int64, bool, error)
	SetUserCountersCache(ctx context.Context, userID int64, counters map[string]int64, ttl time.Duration) error
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

// GetLikedVideoIDs 获取用户点赞的视频id集合
func (r *likeRepository) GetLikedVideoIDs(ctx context.Context, userID int64) ([]int64, error) {
	userLikeKey := fmt.Sprintf("User:Like:Videos:%d", userID) // 严格对齐之前的点赞缓存键
	// A. 优先敲击 Redis 内存盲抠
	exists, err := global.GVA_REDIS.Exists(ctx, userLikeKey).Result()
	if err == nil && exists > 0 {
		// 检查是否存在
		members, err := global.GVA_REDIS.SMembers(ctx, userLikeKey).Result()
		if err == nil && len(members) > 0 {
			ids := make([]int64, 0, len(members))
			for _, m := range members {
				if id, parseErr := strconv.ParseInt(m, 10, 64); parseErr == nil {
					ids = append(ids, id)
				}
			}
			return ids, nil
		}
	}
	// B. 缓存未击中：回源 MySQL 捞取状态为 1 且类型为 video 的点赞明细
	var ids []int64
	err = global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
		Where("user_id = ? AND target_type = ? AND status = 1", userID, "video").
		Order("updated_at DESC").      // 按点赞时间倒序排列
		Pluck("target_id", &ids).Error // target_id的切片

	// C. 顺手回填 Redis 宇宙，赋予 24 小时动态热度寿命
	if err == nil && len(ids) > 0 {
		detachedCtx := context.WithoutCancel(ctx)
		go func(detachedCtx context.Context, LikeIDs []int64) {
			members := make([]interface{}, len(LikeIDs))
			for i, id := range LikeIDs {
				members[i] = id
			}
			pipe := global.GVA_REDIS.Pipeline()
			pipe.SAdd(detachedCtx, userLikeKey, members...)
			pipe.Expire(detachedCtx, userLikeKey, 24*time.Hour)
			_, _ = pipe.Exec(detachedCtx)
		}(detachedCtx, ids)
	}
	return ids, err
}

// StartLikeFlushWorker
// StartLikeFlushWorker ── 🦾 点赞专属永动机：完美修复【流水关系不落盘】与【刷新红心全灭】的终极完全版
func (r *likeRepository) StartLikeFlushWorker(ctx context.Context) {
	defer func() {
		if recoveryErr := recover(); recoveryErr != nil {
			global.LogCtx(ctx).Errorw("🚨 [Panic-Recovery] 后台点赞落库永动机瘫痪，正在全自动重启自愈...", "err", recoveryErr)
			// 发生天塌下来的崩溃时，睡眠1秒，开辟新协程再次重启永动机，保证服务永远不死
			time.Sleep(time.Second)
			go r.StartLikeFlushWorker(context.Background())
		}
	}()

	// ⏱️ 设定平摊大闸：每 10 秒钟疯狂爆发一次批量收网
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// -----------------------------------------------------------------
			// 🛠️ 核心任务一：【点赞流水关系】批量 Upsert 落盘（彻底终结红心变 false 悬案！）
			// -----------------------------------------------------------------
			recordLedgerKey := "list:dirty:like_records"
			rawRecords, err := global.GVA_REDIS.LRange(ctx, recordLedgerKey, 0, 499).Result()
			if err == nil && len(rawRecords) > 0 {
				var insertRecords []pojo.LikeRecord
				for _, raw := range rawRecords {
					parts := strings.Split(raw, ":")
					if len(parts) != 4 {
						continue
					} // 防御损坏的脏账单
					uID, _ := strconv.ParseInt(parts[0], 10, 64)
					tID, _ := strconv.ParseInt(parts[1], 10, 64)
					tType := parts[2]
					status, _ := strconv.Atoi(parts[3])

					insertRecords = append(insertRecords, pojo.LikeRecord{
						UserID: uID, TargetID: tID, TargetType: tType, Status: int8(status),
					})
				}

				if len(insertRecords) > 0 {
					// 💡 核心华彩：利用联合唯一索引解决冲突更新，一枪大批量插入关系流水表！
					txErr := global.GVA_DB.WithContext(ctx).Clauses(clause.OnConflict{
						Columns:   []clause.Column{{Name: "user_id"}, {Name: "target_id"}, {Name: "target_type"}},
						DoUpdates: clause.Assignments(map[string]interface{}{"status": gorm.Expr("VALUES(status)")}),
					}).CreateInBatches(&insertRecords, 100).Error

					if txErr == nil {
						// 只有确认 MySQL 稳妥吞下这批流水关系后，才安全裁剪清理 Redis 队列
						global.GVA_REDIS.LTrim(ctx, recordLedgerKey, int64(len(rawRecords)), -1)
					} else {
						global.LogCtx(ctx).Errorw("❌ [MySQL] 批量焊接点赞流水关系表大翻车", "Error", txErr)
					}
				}
			}

			// -----------------------------------------------------------------
			// 🛠️ 核心任务二：【点赞总数 Count】批量合并削峰 UPDATE（保持你原本的无脑更新）
			// -----------------------------------------------------------------
			dirtyLedgerKey := "mix:dirty:like_targets"
			dirtyTokens, err := global.GVA_REDIS.SPopN(ctx, dirtyLedgerKey, 1000).Result()
			if err != nil || len(dirtyTokens) == 0 {
				continue // 没数字变动，继续闭眼睡觉
			}

			// 多路并网：利用 Pipeline 批量查出最新计数
			pipe := global.GVA_REDIS.Pipeline()
			cmds := make([]*redis.StringCmd, len(dirtyTokens))
			for i, token := range dirtyTokens {
				cmds[i] = pipe.Get(ctx, fmt.Sprintf("Like:Count:%s", token)) //
			}
			_, _ = pipe.Exec(ctx) //

			// 开启 MySQL 大批量焊接落库事务
			tx := global.GVA_DB.Begin() //
			for i, token := range dirtyTokens {
				currentCount, err := cmds[i].Int64() //
				if err != nil {
					continue
				} //

				parts := strings.Split(token, ":") //
				if len(parts) != 2 {
					continue
				} //
				targetType := parts[0]                            //
				targetID, _ := strconv.ParseInt(parts[1], 10, 64) //

				if targetType == "video" {
					tx.Model(&pojo.Video{}).Where("id = ?", targetID).Update("like_count", currentCount) //
				} else if targetType == "comment" {
					tx.Model(&pojo.Comment{}).Where("id = ?", targetID).Update("like_count", currentCount) //
				}
			}
			tx.Commit()                                                                                    //
			global.LogCtx(ctx).Infof("🚀 [Write-Back-Engine] 成功将过去 10 秒累积的 %d 个热点互动合并落库", len(dirtyTokens)) //
		}
	}
}

// MarkTargetAsDirty ── 🚀 纯内存高并发投递大闸
func (r *likeRepository) MarkTargetAsDirty(ctx context.Context, dirtyToken string) error {
	// 1. 定义我们和后台永动机 Worker 约定的秘密账本 Key
	ledgerKey := "mix:dirty:like_targets"

	// 2. 传唤 Redis 执行 SAdd 命令
	err := global.GVA_REDIS.SAdd(ctx, ledgerKey, dirtyToken).Err()
	if err != nil {
		// 记录警告日志
		global.LogCtx(ctx).Errorw("❌ [Redis] 投递脏数据 Token 失败！写缓冲可能出现故障", "token", dirtyToken, "Error", err)
		return err
	}

	return nil
}

func (r *likeRepository) PushLikeRecordQueue(ctx context.Context, payload string) error {
	return global.GVA_REDIS.RPush(ctx, "list:dirty:like_records", payload).Err()
}

// GetAuthorID ── ⚡ 动态锁定创作者 ID
func (r *likeRepository) GetAuthorID(ctx context.Context, targetType string, targetID int64) (int64, error) {
	if targetType == "video" {
		var authorID int64
		// 走覆盖索引，只捞 author_id 字段，极快
		err := global.GVA_DB.WithContext(ctx).Model(&pojo.Video{}).Where("id = ?", targetID).Pluck("author_id", &authorID).Error
		return authorID, err
	} else if targetType == "comment" {
		var userID int64
		// 假设评论表里记录发表人的是 user_id 字段
		err := global.GVA_DB.WithContext(ctx).Table("comments").Where("id = ?", targetID).Pluck("user_id", &userID).Error
		return userID, err
	}
	return 0, errors.New("未知目标类型")
}

// GetUserCountersFromCache ── 🎯 批量提取 Redis Hash 计数器
func (r *likeRepository) GetUserCountersFromCache(ctx context.Context, userID int64) (map[string]int64, bool, error) {
	counterKey := fmt.Sprintf("User:Counters:%d", userID)
	fields := []string{"favorite_count", "total_like", "work_count", "favorite_count_total"} // 对应点赞、获赞、作品、收藏

	vals, err := global.GVA_REDIS.HMGet(ctx, counterKey, fields...).Result()
	if err != nil {
		return nil, false, err
	}

	// 只要有一个字段缺失，就视为未完全命中，触发回源自愈
	for _, v := range vals {
		if v == nil {
			return nil, false, nil
		}
	}

	parseInt := func(i interface{}) int64 {
		s, _ := i.(string)
		res, _ := strconv.ParseInt(s, 10, 64)
		return res
	}

	return map[string]int64{
		"FavoriteCount": parseInt(vals[0]),
		"TotalLike":     parseInt(vals[1]),
		"WorkCount":     parseInt(vals[2]),
		"FavorCount":    parseInt(vals[3]), // 对应你的收藏/点赞数
	}, true, nil
}

// SetUserCountersCache ── 同步 Pipeline 回填
func (r *likeRepository) SetUserCountersCache(ctx context.Context, userID int64, counters map[string]int64, ttl time.Duration) error {
	counterKey := fmt.Sprintf("User:Counters:%d", userID)
	profileMap := map[string]interface{}{
		"favorite_count":       strconv.FormatInt(counters["FavoriteCount"], 10),
		"total_like":           strconv.FormatInt(counters["TotalLike"], 10),
		"work_count":           strconv.FormatInt(counters["WorkCount"], 10),
		"favorite_count_total": strconv.FormatInt(counters["FavorCount"], 10),
	}

	pipe := global.GVA_REDIS.Pipeline()
	pipe.HSet(ctx, counterKey, profileMap)
	pipe.Expire(ctx, counterKey, ttl)
	_, err := pipe.Exec(ctx)
	return err
}
