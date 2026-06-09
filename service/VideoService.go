package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/model/response"
	repo "Go_Project/common/repository"
	"Go_Project/global"
	"Go_Project/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"time"
)

type VideoService struct {
	videoRepo   repo.VideoRepository
	commentRepo repo.CommentRepository
}

// UploadVideoService ── ✅ 视频+封面双 MinIO 落盘与多级缓存同步工厂
func (s *VideoService) UploadVideoService(ctx context.Context, title string, tags string, videoFile, coverFile *multipart.FileHeader, videoObj, coverObj multipart.File, authorID int64, duration int64) error {

	// 🎯 核心升级防线 1：在抛投视频前，确保 "videos" 桶稳如泰山
	if err := utils.EnsureBucketExists(ctx, "videos"); err != nil {
		global.LogCtx(ctx).Errorf("❌ [Service] 确保 videos 存储桶存在失败: %v", err)
		return errors.New("视频存储通道初始化失败，请稍后再试")
	}

	// 1. 将视频源文件推送到 MinIO 的 "videos" 桶中
	videoOk := utils.UpLoadFile(ctx, "videos", videoFile.Filename, videoObj, videoFile.Size)
	if !videoOk {
		global.LogCtx(ctx).Error("❌ [Service] 视频流推入 MinIO 桶失败")
		return errors.New("视频上传存储失败")
	}
	videoUrl := utils.GetFileURL(ctx, "videos", videoFile.Filename, time.Hour*24*7)

	// 🎯 核心升级防线 2：在抛投封面海报前，同样雷达扫描确保 "covers" 桶孵化就位
	if err := utils.EnsureBucketExists(ctx, "covers"); err != nil {
		global.LogCtx(ctx).Errorf("❌ [Service] 确保 covers 存储桶存在失败: %v", err)
		return errors.New("封面图存储通道初始化失败，请稍后再试")
	}

	// 2. 将封面图片推送到 MinIO 的 "covers" 桶中
	coverOk := utils.UpLoadFile(ctx, "covers", coverFile.Filename, coverObj, coverFile.Size)
	if !coverOk {
		global.LogCtx(ctx).Error("❌ [Service] 封面图推入 MinIO 桶失败")
		return errors.New("封面图上传存储失败")
	}
	coverUrl := utils.GetFileURL(ctx, "covers", coverFile.Filename, time.Hour*24*7)

	// 3. 规整核心实体，回源持久化层写入 MySQL
	videoEntity := &pojo.Video{
		Title:         title,
		AuthorID:      authorID, // 锁死当前操作人，坚决防止越权！
		VideoUrl:      videoUrl,
		CoverUrl:      coverUrl,
		Duration:      int(duration),
		LikeCount:     0,
		FavoriteCount: 0,
		Tags:          tags,
	}
	if err := s.videoRepo.CreateVideo(ctx, videoEntity); err != nil {
		return errors.New("视频记录入库失败")
	}

	// 4. 高并发高可用埋点：异步推入 Redis 推荐池
	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, vid int64) {
		poolKey := "GlobalVideoPool"
		err := global.GVA_REDIS.SAdd(traceCtx, poolKey, vid).Err()
		if err != nil {
			global.LogCtx(traceCtx).Errorw("🔥 [Async] 推送新视频至 Redis 推荐池失败", "err", err)
		} else {
			global.LogCtx(traceCtx).Infof("🔥 [Async] 视频 [%d] 已成功并网全局 Redis 推荐池！", vid)
		}
	}(detachedCtx, videoEntity.ID)

	return nil
}

func (s *VideoService) GetVideoFeedService(ctx context.Context) ([]pojo.Video, error) {
	cacheKey := "GlobalFeedCache"

	// 1. 优先拦截并捕获 Redis 内存中的缓存
	cachedData, err := global.GVA_REDIS.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var cachedVideos []pojo.Video
		if json.Unmarshal([]byte(cachedData), &cachedVideos) == nil {
			global.LogCtx(ctx).Infof("[Redis Cache] 内存完美击中！成功拦截并分流了本次 Feed 请求")
			return cachedVideos, nil
		}
	}

	// 2. 🕳️ 穿透/未命中兜底：回源 MySQL
	global.LogCtx(ctx).Warnln("⚠[Redis Cache] 发生缓存穿透/失效！正在紧急回源 MySQL 捞取新鲜流...")
	videos, err := s.videoRepo.GetVideosForFeed(ctx, time.Now(), 4)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [MySQL] 回源查询基础视频流发生毁灭性崩溃: %v", err)
		return nil, err
	}

	// 3. 将新获取的 MySQL 数组序列化成 JSON，回填给 Redis
	if len(videos) > 0 {
		jsonData, err := json.Marshal(videos)
		if err == nil {
			err = global.GVA_REDIS.Set(ctx, cacheKey, jsonData, 10*time.Minute).Err()
			if err != nil {
				global.LogCtx(ctx).Errorw("🔥 [Redis] 回填 Feed 缓存至内存阵列失败", "err", err)
			} else {
				global.LogCtx(ctx).Infoln("✅ [Redis] Feed 缓存已成功并网回填，后续流量正式进入高速公路！")
			}
		}
	}

	return videos, nil
}

// GetFeedStreamService ── ✅ 高性能 Feed 流拼装业务（升级版：消灭 N+1，完美融合最新点赞账本）
func (s *VideoService) GetFeedStreamService(ctx context.Context, currentUserID int64) ([]response.VideoVO, error) {
	// 1. 调用持久层捞取最新发布的时间线原始视频
	pojoVideos, err := s.GetVideoFeedService(ctx)
	if err != nil {
		return nil, err
	}
	videoVOs := make([]response.VideoVO, 0, len(pojoVideos))

	// =================================================================
	// 🎯 核心高并发并网优化：一枪击穿明细表，批量捞取当前操作人的点赞快照
	// =================================================================
	likedVideoMap := make(map[int64]bool)
	if currentUserID > 0 && len(pojoVideos) > 0 {
		// 批量收集当前页面的所有视频 ID
		videoIDs := make([]int64, 0, len(pojoVideos))
		for _, v := range pojoVideos {
			videoIDs = append(videoIDs, v.ID)
		}

		var likedIDs []int64
		// 严格对齐新版幂等点赞账本，只查 status = 1 且类型为 video 的黄金明细
		global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
			Where("user_id = ? AND target_type = ? AND target_id IN ? AND status = 1", currentUserID, "video", videoIDs).
			Pluck("target_id", &likedIDs)

		// 焊接注入内存哈希阵列，实现 O(1) 极速匹配
		for _, id := range likedIDs {
			likedVideoMap[id] = true
		}
	}

	// 2. 穿针引线，开始内存高级组装
	for _, v := range pojoVideos {
		// A. 捞取作者社交卡片
		var author pojo.User
		userKey := fmt.Sprintf("UserProfile:%d", v.AuthorID)
		_ = global.GVA_REDIS.HGetAll(ctx, userKey).Scan(&author)
		if author.ID == 0 {
			global.GVA_DB.First(&author, v.AuthorID)
		}

		vo := response.VideoVO{
			ID:            v.ID,
			Title:         v.Title,
			VideoUrl:      v.VideoUrl,
			CoverUrl:      v.CoverUrl,
			Duration:      v.Duration,
			LikeCount:     v.LikeCount,
			FavoriteCount: v.FavoriteCount,
			CreatedAt:     v.CreatedAt,
			Author: response.AuthorInfo{
				ID:        author.ID,
				Username:  author.Username,
				Avatar:    author.HeadImg,
				Signature: author.Signature,
			},
			IsLike:     likedVideoMap[v.ID], // 🚀 完美对齐内存批量账本，彻底消除刷新熄灭 Bug！
			IsFavorite: false,               // 收藏逻辑未来完全可以镜像同理并网
			Tags:       v.Tags,
			TargetType: "video", // 宣告厂牌，让前端点赞组件盲抠抓取组件上下文
		}
		videoVOs = append(videoVOs, vo)
	}
	return videoVOs, nil
}

// RepairHistoricalDuration ── 回源 MySQL 修正 duration 字段
func (s *VideoService) RepairHistoricalDuration(ctx context.Context, videoID int64, duration int64) error {
	err := global.GVA_DB.Model(&pojo.Video{}).Where("id = ?", videoID).Update("duration", duration).Error
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Repair] 修复历史视频 [%d] 时长失败: %v", videoID, err)
		return err
	}
	global.LogCtx(ctx).Infof("✅ [Repair] 历史数据清洗成功！视频 [%d] 时长已被修正为 %d 秒", videoID, duration)
	return nil
}

// CompleteMultipartVideoService ── 👑 工业级闭环：分片合并 ＋ 数据库物理落盘 ＋ 缓存一致性自愈防御
func (s *VideoService) CompleteMultipartVideoService(ctx context.Context, uploadID, objectName, coverName, title, tags string, duration int64, authorID int64) error {

	// 1. 调用工具层底打，拼装碎片视频
	_, err := utils.MergeMinioMultipartUpload(ctx, "videos", objectName, uploadID)
	if err != nil {
		return errors.New("多媒体切片组装失败，存储通道发生硬熔断")
	}

	// 2. 帮视频和封面海报计算出带有时效的公网高速下载 URL
	videoUrl := utils.GetFileURL(ctx, "videos", objectName, time.Hour*24*7)
	coverUrl := utils.GetFileURL(ctx, "covers", coverName, time.Hour*24*7)

	// 3. 规整核心实体，回源持久化层砸进 MySQL
	videoEntity := &pojo.Video{
		Title:         title,
		AuthorID:      authorID,
		VideoUrl:      videoUrl,
		CoverUrl:      coverUrl,
		Duration:      int(duration),
		LikeCount:     0,
		FavoriteCount: 0,
		Tags:          tags,
	}
	if err := s.videoRepo.CreateVideo(ctx, videoEntity); err != nil {
		return errors.New("视频记录并网入库失败")
	}

	// 4. 🔥 数据一致性防御大闸门：清除 Feed 缓存与作者作品集缓存
	_ = global.GVA_REDIS.Del(ctx, "GlobalFeedCache").Err()
	global.LogCtx(ctx).Infoln("🧹 [Cache Eviction] 直传合并成功，已全自动清空老旧 Redis 首页大 Feed 缓存层！")
	userCacheKey := fmt.Sprintf("UserVideoList:%d", authorID)
	_ = global.GVA_REDIS.Del(ctx, userCacheKey).Err()

	// 5. 高并发异步埋点：推入全局 Redis 推荐池
	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, vid int64) {
		poolKey := "GlobalVideoPool"
		_ = global.GVA_REDIS.SAdd(traceCtx, poolKey, vid).Err()
	}(detachedCtx, videoEntity.ID)
	return nil
}

// GetUserVideoListService ── 👑 接入动态 Redis 缓存的个人作品集装配业务
func (s *VideoService) GetUserVideoListService(ctx context.Context, targetUserID int64) ([]response.VideoVO, error) {
	cacheKey := fmt.Sprintf("UserVideoList:%d", targetUserID)

	// 1. 探针雷达优先拦截内存
	cachedData, err := global.GVA_REDIS.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var cachedVOs []response.VideoVO
		if json.Unmarshal([]byte(cachedData), &cachedVOs) == nil {
			global.LogCtx(ctx).Infof("⚡ [Redis] 成功击中创作者 [%d] 的个人主页作品集缓存！", targetUserID)
			return cachedVOs, nil
		}
	}

	// 2. 未命中兜底：回源 MySQL
	global.LogCtx(ctx).Warnf("⚠️ [Redis] 创作者 [%d] 个人缓存穿透，正在回源 MySQL 补货...", targetUserID)
	pojoVideos, err := s.videoRepo.GetVideosByAuthorID(ctx, targetUserID)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [MySQL] 捞取用户作品集发生毁灭性崩溃: %v", err)
		return nil, err
	}

	// 3. 开始拼装社交 VO 实体
	var author pojo.User
	userKey := fmt.Sprintf("UserProfile:%d", targetUserID)
	_ = global.GVA_REDIS.HGetAll(ctx, userKey).Scan(&author)
	if author.ID == 0 {
		global.GVA_DB.First(&author, targetUserID)
	}

	videoVOs := make([]response.VideoVO, 0, len(pojoVideos))
	for _, v := range pojoVideos {
		vo := response.VideoVO{
			ID:            v.ID,
			Title:         v.Title,
			VideoUrl:      v.VideoUrl,
			CoverUrl:      v.CoverUrl,
			Duration:      v.Duration,
			LikeCount:     v.LikeCount,
			FavoriteCount: v.FavoriteCount,
			CreatedAt:     v.CreatedAt,
			Tags:          v.Tags,
			Author: response.AuthorInfo{
				ID:        author.ID,
				Username:  author.Username,
				Avatar:    author.HeadImg,
				Signature: author.Signature,
			},
			TargetType: "video", // 顺手补齐实体厂牌标识
		}
		videoVOs = append(videoVOs, vo)
	}

	// 4. 送回 Redis，设置 10 分钟弹性时效
	if len(videoVOs) > 0 {
		jsonData, err := json.Marshal(videoVOs)
		if err == nil {
			_ = global.GVA_REDIS.Set(ctx, cacheKey, jsonData, 10*time.Minute).Err()
		}
	}

	return videoVOs, nil
}

// GetVideoDetailService ── 👑 视频全景视图：单体信息 + 社交卡片 + 千人千面状态装配
func (s *VideoService) GetVideoDetailService(ctx context.Context, videoID int64, currentUserID int64) (*response.VideoVO, error) {

	// 1. 底层单点击穿：捞取视频基础流信息
	video, err := s.videoRepo.GetVideoByID(ctx, videoID)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [MySQL] 捞取单体视频 [%d] 遭遇滑铁卢: %v", videoID, err)
		return nil, errors.New("视频可能已迷失")
	}

	// 2. 社交作者卡片并网：优先走 Redis 高速动态哈希
	var author pojo.User
	userKey := fmt.Sprintf("UserProfile:%d", video.AuthorID)
	_ = global.GVA_REDIS.HGetAll(ctx, userKey).Scan(&author)
	if author.ID == 0 {
		global.GVA_DB.First(&author, video.AuthorID)
	}
	likeCountKey := fmt.Sprintf("Like:Count:video:%d", videoID)
	cacheCount, err := global.GVA_REDIS.Get(ctx, likeCountKey).Int64()
	if err != nil {
		cacheCount = video.LikeCount
		global.GVA_REDIS.Set(ctx, likeCountKey, cacheCount, 24*time.Hour)
	}
	// 3. 千人千面交互感知：升级为精准敲门 MySQL 新点赞明细表
	var isLike = false
	if currentUserID > 0 {
		likeSetKey := fmt.Sprintf("Like:Set:video:%d", videoID)
		exists, err := global.GVA_REDIS.SIsMember(ctx, likeSetKey, currentUserID).Result()
		if err != nil {
			isLike = exists
		} else {
			var count int64
			global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
				Where("user_id = ? AND target_id = ? AND target_type = ? AND status = 1", currentUserID, videoID, "video").
				Count(&count)
			isLike = count > 0
		}
		if isLike {
			global.GVA_REDIS.SAdd(ctx, likeSetKey, currentUserID)
		}
	}
	// 4. 契约规整：打包成我们前端 Vue 3 极度渴望的奢华 VO 对象
	vo := &response.VideoVO{
		ID:            video.ID,
		Title:         video.Title,
		VideoUrl:      video.VideoUrl,
		CoverUrl:      video.CoverUrl,
		Duration:      video.Duration,
		LikeCount:     video.LikeCount, // 承接底层原子计数器的行锁安全数字
		FavoriteCount: video.FavoriteCount,
		CreatedAt:     video.CreatedAt,
		Tags:          video.Tags,
		Author: response.AuthorInfo{
			ID:        author.ID,
			Username:  author.Username,
			Avatar:    author.HeadImg,
			Signature: author.Signature,
		},
		IsLike:     isLike, // 精准回显状态，刷新坚决不灭！
		IsFavorite: false,
		TargetType: "video",
	}

	return vo, nil
}
