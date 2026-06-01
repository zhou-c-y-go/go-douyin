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
func (s *VideoService) UploadVideoService(ctx context.Context, title string, videoFile, coverFile *multipart.FileHeader, videoObj, coverObj multipart.File, authorID int64) error {
	// 1. 将视频源文件推送到 MinIO 的 "videos" 桶中
	videoOk := utils.UpLoadFile("videos", videoFile.Filename, videoObj, videoFile.Size)
	if !videoOk {
		global.LogCtx(ctx).Error("❌ [Service] 视频流推入 MinIO 桶失败")
		return errors.New("视频上传存储失败")
	}
	videoUrl := utils.GetFileURL("videos", videoFile.Filename, time.Hour*24*7)
	// 2. 将封面图片推送到 MinIO 的 "covers" 桶中
	coverOk := utils.UpLoadFile("covers", coverFile.Filename, coverObj, coverFile.Size)
	if !coverOk {
		global.LogCtx(ctx).Error("❌ [Service] 封面图推入 MinIO 桶失败")
		return errors.New("封面图上传存储失败")
	}
	coverUrl := utils.GetFileURL("covers", coverFile.Filename, time.Hour*24*7)
	// 3. 规整核心实体，回源持久化层写入 MySQL
	videoEntity := &pojo.Video{
		Title:         title,
		AuthorID:      authorID, // 锁死当前操作人，坚决防止越权！
		VideoUrl:      videoUrl,
		CoverUrl:      coverUrl,
		Duration:      0, // 实际开发中可以通过 ffmpeg 提取，此处先默认为 0
		LikeCount:     0,
		FavoriteCount: 0,
	}
	if err := s.videoRepo.CreateVideo(ctx, videoEntity); err != nil {
		return errors.New("视频记录入库失败")
	}
	// 4. 💡 高并发高可用埋点：利用 WithoutCancel 异步将新视频 ID 推入 Redis 全局新鲜推荐池！
	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, vid int64) {
		poolKey := "GlobalVideoPool"
		// 将新发布的视频 ID 塞进 Redis 集合，供推荐流秒级盲盒抓取
		err := global.GVA_REDIS.SAdd(traceCtx, poolKey, vid).Err()
		if err != nil {
			global.LogCtx(traceCtx).Errorw("🔥 [Async] 推送新视频至 Redis 推荐池失败", "err", err)
		} else {
			global.LogCtx(traceCtx).Infof("🔥 [Async] 视频 [%d] 已成功并网全局 Redis 推荐池！", vid)
		}
	}(detachedCtx, videoEntity.ID)
	return nil
}

// GetFeedStreamService ── ✅ 乐高式高性能 Feed 流拼装业务
func (s *VideoService) GetFeedStreamService(ctx context.Context, currentUserID int64) ([]response.VideoVO, error) {
	// 1. 调用持久层捞取最新发布的时间线原始视频（此处默认限流 5 条）
	pojoVideos, err := s.videoRepo.GetVideosForFeed(ctx, time.Now(), 5)
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
		// C. 乐高积木合体
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
		}
		videoVOs = append(videoVOs, vo)
	}
	return videoVOs, nil
}
