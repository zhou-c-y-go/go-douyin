package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/model/response"
	repo "Go_Project/common/repository"
	"Go_Project/global"
	"Go_Project/utils"
	"context"
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

	// 🎯 核心升级防线 1：在抛投视频前，调用 v7 自动化雷达，确保 "videos" 桶稳如泰山（不存在则自动建桶并注入公网可读 JSON）
	if err := utils.EnsureBucketExists(ctx, "videos"); err != nil {
		global.LogCtx(ctx).Errorf("❌ [Service] 确保 videos 存储桶存在失败: %v", err)
		return errors.New("视频存储通道初始化失败，请稍后再试")
	}

	// 1. 将视频源文件推送到 MinIO 的 "videos" 桶中（💡 注意：顺水推舟把 ctx 传进去，开启断线熔断保护！）
	videoOk := utils.UpLoadFile(ctx, "videos", videoFile.Filename, videoObj, videoFile.Size)
	if !videoOk {
		global.LogCtx(ctx).Error("❌ [Service] 视频流推入 MinIO 桶失败")
		return errors.New("视频上传存储失败")
	}
	// 获取下载链接时同样统一流转 ctx
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
		Duration:      int(duration), // 实际开发中可以通过 ffmpeg 提取，此处先默认为 0
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

// GetFeedStreamService ── ✅ 高性能 Feed 流拼装业务
func (s *VideoService) GetFeedStreamService(ctx context.Context, currentUserID int64) ([]response.VideoVO, error) {
	// 1. 调用持久层捞取最新发布的时间线原始视频（此处默认限流 5 条）
	pojoVideos, err := s.videoRepo.GetVideosForFeed(ctx, time.Now(), 4)
	if err != nil {
		return nil, err
	}
	videoVOs := make([]response.VideoVO, 0, len(pojoVideos))

	// 2. 穿针引线，开始内存高级组装
	for _, v := range pojoVideos {
		// A. 捞取作者社交卡片：优先走 Redis 缓存，无缓存回源 MySQL
		var author pojo.User
		userKey := fmt.Sprintf("UserProfile:%d", v.AuthorID)
		err := global.GVA_REDIS.HGetAll(ctx, userKey).Scan(&author)
		if err != nil {
			return nil, err
		}
		if author.ID == 0 {
			global.GVA_DB.First(&author, v.AuthorID)
		}
		// B. 捞取千人千面动态交互状态：查询当前用户是否点赞/收藏过该视频
		var isLike bool
		if currentUserID > 0 {
			likeKey := fmt.Sprintf("UserLikes:%d", currentUserID)
			isLike, _ = global.GVA_REDIS.SIsMember(ctx, likeKey, v.ID).Result()
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
				Avatar:    author.HeadImg, // 完美衔接 MinIO 头像
				Signature: author.Signature,
			},
			IsLike:     isLike,
			IsFavorite: false, // 收藏逻辑完全镜像同理
			Tags:       v.Tags,
		}
		videoVOs = append(videoVOs, vo)
	}
	return videoVOs, nil
}

// RepairHistoricalDuration ── 回源 MySQL 修正 duration 字段
func (s *VideoService) RepairHistoricalDuration(ctx context.Context, videoID int64, duration int64) error {
	// 直接唤醒 GORM 执行原子更新，把历史遗留的 0 秒强刷为真实秒数
	err := global.GVA_DB.Model(&pojo.Video{}).Where("id = ?", videoID).Update("duration", duration).Error
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Repair] 修复历史视频 [%d] 时长失败: %v", videoID, err)
		return err
	}

	global.LogCtx(ctx).Infof("✅ [Repair] 历史数据清洗成功！视频 [%d] 时长已被修正为 %d 秒", videoID, duration)
	return nil
}
