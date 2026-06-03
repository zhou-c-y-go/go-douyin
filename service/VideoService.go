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

func (s *VideoService) GetVideoFeedService(ctx context.Context) ([]pojo.Video, error) {
	cacheKey := "GlobalFeedCache"

	// 1. 优先拦截并捕获 Redis 内存中的缓存
	cachedData, err := global.GVA_REDIS.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		// 反序列化 JSON 字符串
		var cachedVideos []pojo.Video
		if json.Unmarshal([]byte(cachedData), &cachedVideos) == nil {
			global.LogCtx(ctx).Infof("[Redis Cache] 内存完美击中！成功拦截并分流了本次 Feed 请求")
			return cachedVideos, nil
		}
	}
	// 2. 🕳️ 穿透/未命中兜底：如果 Redis 没捞到，说明是冷数据，老老实实回源 MySQL
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
			// 设置过期时间（TTL），这里设置 10 分钟弹性缓冲区
			// 配合 time.Duration，防范僵尸脏数据长期霸占内存
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

// GetFeedStreamService ── ✅ 高性能 Feed 流拼装业务
func (s *VideoService) GetFeedStreamService(ctx context.Context, currentUserID int64) ([]response.VideoVO, error) {
	// 1. 调用持久层捞取最新发布的时间线原始视频（此处默认限流 5 条）
	pojoVideos, err := s.GetVideoFeedService(ctx)
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

// CompleteMultipartVideoService ── 👑 工业级闭环：分片合并 ＋ 数据库物理落盘 ＋ 缓存一致性自愈防御
func (s *VideoService) CompleteMultipartVideoService(ctx context.Context, uploadID, objectName, coverName, title, tags string, duration int64, authorID int64) error {

	// 1. 调用工具层底打，命令 MinIO 引擎在底层迅速拼装碎片视频
	_, err := utils.MergeMinioMultipartUpload(ctx, "videos", objectName, uploadID)
	if err != nil {
		return errors.New("多媒体切片组装失败，存储通道发生硬熔断")
	}

	// 2. 借助我们已有的雷达，帮视频和封面海报计算出带有时效的公网高速下载 URL
	videoUrl := utils.GetFileURL(ctx, "videos", objectName, time.Hour*24*7) //
	coverUrl := utils.GetFileURL(ctx, "covers", coverName, time.Hour*24*7)  //

	// 3. 规整核心实体，回源持久化层砸进 MySQL
	videoEntity := &pojo.Video{
		Title:         title,         //
		AuthorID:      authorID,      // 锁死当前发帖人
		VideoUrl:      videoUrl,      //
		CoverUrl:      coverUrl,      //
		Duration:      int(duration), // 注入本地前置雷达捕获的精准秒数
		LikeCount:     0,             //
		FavoriteCount: 0,             //
		Tags:          tags,          //
	}
	if err := s.videoRepo.CreateVideo(ctx, videoEntity); err != nil { //
		return errors.New("视频记录并网入库失败") //
	}

	// 4. 🔥 数据一致性防御大闸门：既然有尊贵的新作品发布了，全自动静默擦除老旧的首页 Feed 缓存区！
	_ = global.GVA_REDIS.Del(ctx, "GlobalFeedCache").Err()
	global.LogCtx(ctx).Infoln("🧹 [Cache Eviction] 直传合并成功，已全自动清空老旧 Redis 首页大 Feed 缓存层！")
	userCacheKey := fmt.Sprintf("UserVideoList:%d", authorID)
	_ = global.GVA_REDIS.Del(ctx, userCacheKey).Err()
	// 5. 高并发异步埋点：把生成的视频自增 ID 抛投给后台协程，无感推入全局 Redis 推荐池
	detachedCtx := context.WithoutCancel(ctx)      //
	go func(traceCtx context.Context, vid int64) { //
		poolKey := "GlobalVideoPool"                            //
		_ = global.GVA_REDIS.SAdd(traceCtx, poolKey, vid).Err() //
	}(detachedCtx, videoEntity.ID) //
	return nil
}

// GetUserVideoListService ── 👑 接入动态 Redis 缓存的个人作品集装配业务
func (s *VideoService) GetUserVideoListService(ctx context.Context, targetUserID int64) ([]response.VideoVO, error) {
	// 🎯 核心微操：针对不同的创作者，孵化出独一无二的专属内存 Key
	cacheKey := fmt.Sprintf("UserVideoList:%d", targetUserID)
	// 1. 探针雷达优先拦截内存，击中则 切流返回
	cachedData, err := global.GVA_REDIS.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var cachedVOs []response.VideoVO
		if json.Unmarshal([]byte(cachedData), &cachedVOs) == nil {
			global.LogCtx(ctx).Infof("⚡ [Redis] 成功击中创作者 [%d] 的个人主页作品集缓存！", targetUserID)
			return cachedVOs, nil
		}
	}

	// 2. 未命中兜底：回源持久层去磁盘上找数据
	global.LogCtx(ctx).Warnf("⚠️ [Redis] 创作者 [%d] 个人缓存穿透，正在回源 MySQL 补货...", targetUserID)
	pojoVideos, err := s.videoRepo.GetVideosByAuthorID(ctx, targetUserID)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [MySQL] 捞取用户作品集发生毁灭性崩溃: %v", err)
		return nil, err
	}

	// 3. 开始拼装精美的社交 VO 实体（复用主页的 AuthorInfo 结构，方便前端组件完美渲染）
	// 顺手捞取一次作者的基础卡片（优先走 Redis 动态哈希）
	var author pojo.User
	userKey := fmt.Sprintf("UserProfile:%d", targetUserID)   //
	_ = global.GVA_REDIS.HGetAll(ctx, userKey).Scan(&author) //
	if author.ID == 0 {
		global.GVA_DB.First(&author, targetUserID) //
	}

	videoVOs := make([]response.VideoVO, 0, len(pojoVideos)) //
	for _, v := range pojoVideos {
		vo := response.VideoVO{ //
			ID:            v.ID,            //
			Title:         v.Title,         //
			VideoUrl:      v.VideoUrl,      //
			CoverUrl:      v.CoverUrl,      //
			Duration:      v.Duration,      //
			LikeCount:     v.LikeCount,     //
			FavoriteCount: v.FavoriteCount, //
			CreatedAt:     v.CreatedAt,     //
			Tags:          v.Tags,          //
			Author: response.AuthorInfo{ //
				ID:        author.ID,        //
				Username:  author.Username,  //
				Avatar:    author.HeadImg,   //
				Signature: author.Signature, //
			},
		}
		videoVOs = append(videoVOs, vo) //
	}

	// 4. 将装配好的 VO 数组送回 Redis，设置10分钟弹性时效
	if len(videoVOs) > 0 {
		jsonData, err := json.Marshal(videoVOs)
		if err == nil {
			_ = global.GVA_REDIS.Set(ctx, cacheKey, jsonData, 10*time.Minute).Err() //
		}
	}

	return videoVOs, nil
}
