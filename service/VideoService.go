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
	"github.com/redis/go-redis/v9"
	"mime/multipart"
	"time"
)

// VideoService 视频模块业务标杆接口
type VideoService interface {
	UploadVideoService(ctx context.Context, title string, tags string, videoFile, coverFile *multipart.FileHeader, videoObj, coverObj multipart.File, authorID, duration int64) error
	GetVideoFeedService(ctx context.Context) ([]pojo.Video, error)
	GetFeedStreamService(ctx context.Context, currentUserID int64) ([]response.VideoVO, error)
	RepairHistoricalDuration(ctx context.Context, videoID int64, duration int64) error
	CompleteMultipartVideoService(ctx context.Context, uploadID, objectName, coverName, title, tags string, duration int64, authorID int64) error
	GetUserVideoListService(ctx context.Context, targetUserID int64) ([]response.VideoVO, error)
	GetVideoDetailService(ctx context.Context, videoID int64, currentUserID int64) (*response.VideoVO, error)
	GetUserTotalWorkCountService(ctx context.Context, authorID int64) (int64, error)
	GetUserLikeVideoListService(ctx context.Context, targetUserID int64, currentUserID int64) ([]response.VideoVO, error)
	GetUserFavoriteVideoListService(ctx context.Context, targetUserID int64, currentUserID int64) ([]response.VideoVO, error)
}

type videoService struct {
	videoRepo repo.VideoRepository
	userRepo  repo.UserRepository // 引入以支持内存多表并网聚合
	likeRepo  repo.LikeRepository
	favRepo   repo.FavoriteRepository
}

func NewVideoService(vr repo.VideoRepository, ur repo.UserRepository, lr repo.LikeRepository, fr repo.FavoriteRepository) VideoService {
	return &videoService{
		videoRepo: vr,
		userRepo:  ur,
		likeRepo:  lr,
		favRepo:   fr,
	}
}

func (s *videoService) UploadVideoService(ctx context.Context, title string, tags string, videoFile, coverFile *multipart.FileHeader, videoObj, coverObj multipart.File, authorID, duration int64) error {
	if err := utils.EnsureBucketExists(ctx, "videos"); err != nil {
		return errors.New("视频存储通道初始化失败")
	}
	if !utils.UpLoadFile(ctx, "videos", videoFile.Filename, videoObj, videoFile.Size) {
		return errors.New("视频上传存储失败")
	}
	videoUrl := utils.GetFileURL(ctx, "videos", videoFile.Filename, time.Hour*24*7)

	if err := utils.EnsureBucketExists(ctx, "covers"); err != nil {
		return errors.New("封面图存储通道初始化失败")
	}
	if !utils.UpLoadFile(ctx, "covers", coverFile.Filename, coverObj, coverFile.Size) {
		return errors.New("封面图上传存储失败")
	}
	coverUrl := utils.GetFileURL(ctx, "covers", coverFile.Filename, time.Hour*24*7)

	videoEntity := &pojo.Video{
		Title: title, AuthorID: authorID, VideoUrl: videoUrl, CoverUrl: coverUrl,
		Duration: int(duration), LikeCount: 0, FavoriteCount: 0, Tags: tags,
	}
	if err := s.videoRepo.CreateVideo(ctx, videoEntity); err != nil {
		return errors.New("视频记录入库失败")
	}

	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, vid, uid int64) {
		_ = s.videoRepo.AddVideoToPoolCache(traceCtx, vid)
		_ = s.videoRepo.DelUserVideoListCache(traceCtx, uid)
		workCountKey := fmt.Sprintf("Video:Count:author:%d", authorID)
		global.GVA_REDIS.Del(traceCtx, workCountKey)
	}(detachedCtx, videoEntity.ID, authorID)
	return nil
}

func (s *videoService) GetVideoFeedService(ctx context.Context) ([]pojo.Video, error) {
	cachedData, err := s.videoRepo.GetGlobalFeedCache(ctx)
	if err == nil && cachedData != "" {
		var cachedVideos []pojo.Video
		if json.Unmarshal([]byte(cachedData), &cachedVideos) == nil {
			return cachedVideos, nil
		}
	}

	videos, err := s.videoRepo.GetVideosForFeed(ctx, time.Now(), 4)
	if err != nil {
		return nil, err
	}
	if len(videos) > 0 {
		jsonData, _ := json.Marshal(videos)
		_ = s.videoRepo.SetGlobalFeedCache(ctx, string(jsonData), 10*time.Second)
	}
	return videos, nil
}

func (s *videoService) GetFeedStreamService(ctx context.Context, currentUserID int64) ([]response.VideoVO, error) {
	pojoVideos, err := s.GetVideoFeedService(ctx)
	if err != nil {
		return nil, err
	}
	videoVOs := make([]response.VideoVO, 0, len(pojoVideos))

	likedVideoMap := make(map[int64]bool)
	if currentUserID > 0 && len(pojoVideos) > 0 {
		videoIDs := make([]int64, 0, len(pojoVideos))
		for _, v := range pojoVideos {
			videoIDs = append(videoIDs, v.ID)
		}
		likedIDs, _ := s.videoRepo.BatchGetLikedVideoIDs(ctx, currentUserID, videoIDs)
		for _, id := range likedIDs {
			likedVideoMap[id] = true
		}
	}

	for _, v := range pojoVideos {
		// 回源依赖注入的 userRepo 聚合用户信息
		var author pojo.User
		fields, err := s.userRepo.GetProfileCache(ctx, v.AuthorID)
		if err == nil && fields["Username"] != "" {
			author.ID = v.AuthorID
			author.Username = fields["Username"]
			author.HeadImg = fields["HeadImg"]
			author.Signature = fields["Signature"]
		} else {
			author, _ = s.userRepo.QueryByID(ctx, int(v.AuthorID))
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
				ID: author.ID, Username: author.Username, Avatar: author.HeadImg, Signature: author.Signature,
			},
			IsLike:     likedVideoMap[v.ID],
			IsFavorite: likedVideoMap[v.ID],
			Tags:       v.Tags,
			TargetType: "video",
		}
		videoVOs = append(videoVOs, vo)
	}
	return videoVOs, nil
}

func (s *videoService) RepairHistoricalDuration(ctx context.Context, videoID int64, duration int64) error {
	return s.videoRepo.UpdateDuration(ctx, videoID, duration)
}

func (s *videoService) CompleteMultipartVideoService(ctx context.Context, uploadID, objectName, coverName, title, tags string, duration int64, authorID int64) error {
	_, err := utils.MergeMinioMultipartUpload(ctx, "videos", objectName, uploadID)
	if err != nil {
		return errors.New("多媒体切片组装失败")
	}

	videoUrl := utils.GetFileURL(ctx, "videos", objectName, time.Hour*24*7)
	coverUrl := utils.GetFileURL(ctx, "covers", coverName, time.Hour*24*7)

	videoEntity := &pojo.Video{
		Title: title, AuthorID: authorID, VideoUrl: videoUrl, CoverUrl: coverUrl,
		Duration: int(duration), LikeCount: 0, FavoriteCount: 0, Tags: tags,
	}
	if err = s.videoRepo.CreateVideo(ctx, videoEntity); err != nil {
		return errors.New("视频记录并网入库失败")
	}

	_ = s.videoRepo.DelGlobalFeedCache(ctx)
	_ = s.videoRepo.DelUserVideoListCache(ctx, authorID)

	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, vid int64) {
		_ = s.videoRepo.AddVideoToPoolCache(traceCtx, vid)
		workCountKey := fmt.Sprintf("Video:Count:author:%d", authorID)
		global.GVA_REDIS.Del(traceCtx, workCountKey)
	}(detachedCtx, videoEntity.ID)
	return nil
}

// GetUserVideoListService 获取用户作品集
func (s *videoService) GetUserVideoListService(ctx context.Context, targetUserID int64) ([]response.VideoVO, error) {
	// 1. ⚡ 缓存大闸拦截
	cachedData, err := s.videoRepo.GetUserVideoListCache(ctx, targetUserID)
	if err == nil && cachedData != "" {
		var cachedVOs []response.VideoVO
		if json.Unmarshal([]byte(cachedData), &cachedVOs) == nil {
			return cachedVOs, nil // 完美命中缓存，直接发货
		}
	}
	global.LogCtx(ctx).Infof("ℹ️ [Cache-Miss] 用户 %d 的视频列表缓存未命中，正在动态回源 MySQL...", targetUserID)

	// 2. 回源 MySQL 捞取该作者的视频元数据
	pojoVideos, err := s.videoRepo.GetVideosByAuthorID(ctx, targetUserID)
	if err != nil {
		global.LogCtx(ctx).Errorw("❌ [MySQL] 捞取作者视频列表大翻车", "userID", targetUserID, "Error", err)
		return nil, err
	}

	// 如果连数据库也是空的，直接返回空切片，没必要往下忙活了
	if len(pojoVideos) == 0 {
		return []response.VideoVO{}, nil
	}

	// 3. 捞取作者静态资料（优先走 Redis 拦截）
	var author pojo.User
	fields, _ := s.userRepo.GetProfileCache(ctx, targetUserID)
	if fields["Username"] != "" {
		author.ID = targetUserID
		author.Username = fields["Username"]
		author.HeadImg = fields["HeadImg"]
		author.Signature = fields["Signature"]
	} else {
		author, _ = s.userRepo.QueryByID(ctx, int(targetUserID))
	}

	// =========================================================================
	// 🦾【大厂原子微操】：利用 Pipeline 矩阵，一枪端回所有视频的最新热度计数
	// 彻底消灭循环内反复敲击 Redis 的 40 次网络 I/O 恶习！
	// =========================================================================
	pipe := global.GVA_REDIS.Pipeline()
	likeCmds := make([]*redis.StringCmd, len(pojoVideos))
	favCmds := make([]*redis.StringCmd, len(pojoVideos))

	for i, v := range pojoVideos {
		likeCmds[i] = pipe.Get(ctx, fmt.Sprintf("Like:Count:video:%d", v.ID))
		favCmds[i] = pipe.Get(ctx, fmt.Sprintf("Favor:Count:video:%d", v.ID)) // 对应你的收藏键名
	}
	_, _ = pipe.Exec(ctx) // 1次网络交互，收网所有数字！

	// 4. 内存装配
	videoVOs := make([]response.VideoVO, 0, len(pojoVideos))
	for i, v := range pojoVideos {
		// 从 Pipeline 结果中抠数字，抠不到（err）就拿 MySQL 永动机同步的冷数字保底
		likeCount, lErr := likeCmds[i].Int64()
		if lErr != nil {
			likeCount = v.LikeCount
		}

		favCount, fErr := favCmds[i].Int64()
		if fErr != nil {
			favCount = v.FavoriteCount
		}

		vo := response.VideoVO{
			ID: v.ID, Title: v.Title, VideoUrl: v.VideoUrl, CoverUrl: v.CoverUrl,
			Duration: v.Duration, LikeCount: likeCount, FavoriteCount: favCount, CreatedAt: v.CreatedAt, Tags: v.Tags,
			Author: response.AuthorInfo{
				ID: author.ID, Username: author.Username, Avatar: author.HeadImg, Signature: author.Signature,
			},
			TargetType: "video",
		}
		videoVOs = append(videoVOs, vo)
	}
	if len(videoVOs) > 0 {
		jsonData, _ := json.Marshal(videoVOs)
		// 赋予 10 分钟缓存寿命
		_ = s.videoRepo.SetUserVideoListCache(ctx, targetUserID, string(jsonData), 10*time.Minute)
		global.LogCtx(ctx).Infof("🚀 [Cache-Rebuild] 成功为用户 %d 重建了视频列表缓存大衣", targetUserID)
	}

	return videoVOs, nil
}

func (s *videoService) GetVideoDetailService(ctx context.Context, videoID int64, currentUserID int64) (*response.VideoVO, error) {
	// 优先从 Redis 批量反序列化捞取视频静态快照 (Title, VideoUrl, CoverUrl 等)
	var video *pojo.Video
	var videoHit bool
	jsonStr, err := s.videoRepo.GetUserVideoListCache(ctx, videoID)
	if errors.Is(err, redis.Nil) {
		videoHit = false
		global.LogCtx(ctx).Infow("💡 [Cache] 视频详情缓存未击中，准备回源 MySQL", "videoID", videoID)
	} else if err != nil {
		// 真实的 Redis 瘫痪或网络故障，降级处理
		videoHit = false
		global.LogCtx(ctx).Errorw("❌ [Redis] 调取视频详情缓存发生系统级崩溃", "videoID", videoID, "Error", err)
	} else {
		// 3. 脏活累活 Service 做：亲自进行反序列化 (Unmarshal)
		video = &pojo.Video{}
		if marshalErr := json.Unmarshal([]byte(jsonStr), video); marshalErr != nil {
			global.LogCtx(ctx).Errorw("❌ [Json] 视频缓存 JSON 格式基因突变，反序列化失败", "videoID", videoID, "Error", marshalErr)
			videoHit = false // 格式错误，视为未击中，强制回源
		} else {
			videoHit = true
		}
	}

	if !videoHit {
		// 🚨 缓存未击中：被迫下沉回源 MySQL 查明真身
		video, err = s.videoRepo.GetVideoByID(ctx, videoID)
		if err != nil || video == nil {
			global.LogCtx(ctx).Errorw("❌ [MySQL] 目标视频实体已彻底在人间迷失", "videoID", videoID, "Error", err)
			return nil, errors.New("视频可能已迷失")
		}

		// 🚀 赛博异步分流：剥离掉取消信号后，开辟独立协程后台回填 Redis，绝不卡住主响应线程
		detachedCtx := context.WithoutCancel(ctx)
		go func(bgCtx context.Context, vid int64, vData *pojo.Video) {
			vBytes, _ := json.Marshal(vData)
			_ = s.videoRepo.SetUserVideoListCache(bgCtx, vid, string(vBytes), 24*time.Hour)
		}(detachedCtx, videoID, video)
	}

	// =========================================================================
	// 🎯 第二方阵：👤 聚合作者空间名片 (保持缓存优先设计)
	// =========================================================================
	var author pojo.User
	fields, _ := s.userRepo.GetProfileCache(ctx, video.AuthorID)
	if fields["Username"] != "" {
		author.ID = video.AuthorID
		author.Username = fields["Username"]
		author.HeadImg = fields["HeadImg"]
		author.Signature = fields["Signature"]
	} else {
		author, _ = s.userRepo.QueryByID(ctx, int(video.AuthorID))
	}

	// =========================================================================
	// 🎯 第三方阵：📈 抓取独立高频动态计数器 (点赞数 / 收藏数)
	// 💡 架构微操：由于计数器变化极其剧烈，与基础静态资料做物理隔离，独立走 Redis String 拦截
	// =========================================================================
	cacheLikeCount, err := s.videoRepo.GetVideoLikeCountCache(ctx, videoID)
	if err != nil {
		cacheLikeCount = video.LikeCount
		detachedCtx := context.WithoutCancel(ctx)
		go func(bgCtx context.Context) {
			_ = s.videoRepo.SetVideoLikeCountCache(bgCtx, videoID, cacheLikeCount, 24*time.Hour)
		}(detachedCtx)
	}

	cacheFavoriteCount, err := s.videoRepo.GetVideoFavoriteCountCache(ctx, videoID)
	if err != nil {
		cacheFavoriteCount = video.FavoriteCount
		detachedCtx := context.WithoutCancel(ctx)
		go func(bgCtx context.Context) {
			_ = s.videoRepo.SetVideoFavoriteCountCache(bgCtx, videoID, cacheFavoriteCount, 24*time.Hour)
		}(detachedCtx)
	}

	// =========================================================================
	// 🎯 第四方阵：🎛️ 千人千面交互状态链并网 (IsLike / IsFavorite)
	// =========================================================================
	var isLike, isFavor bool
	if currentUserID > 0 {
		// A. 动态研判当前登录用户是否对该视频点过赞
		keyExists, err := s.videoRepo.ExistsLikeSetCache(ctx, videoID)
		if err != nil || keyExists == 0 {
			isLike, _ = s.videoRepo.CheckLikeRecordExists(ctx, currentUserID, videoID)
			if isLike {
				detachedCtx := context.WithoutCancel(ctx)
				go func(bgCtx context.Context) {
					_ = s.videoRepo.AddLikeSetCache(bgCtx, videoID, currentUserID, 24*time.Hour)
				}(detachedCtx)
			}
		} else {
			isLike, _ = s.videoRepo.IsMemberLikeSetCache(ctx, videoID, currentUserID)
		}

		// B. 动态研判当前登录用户是否对该视频进行过悄悄收藏
		favKeyExists, err := s.videoRepo.ExistsFavoriteSetCache(ctx, videoID)
		if err != nil || favKeyExists == 0 {
			isFavor, _ = s.videoRepo.CheckFavoriteRecordExists(ctx, currentUserID, videoID)
			if isFavor {
				detachedCtx := context.WithoutCancel(ctx)
				go func(bgCtx context.Context) {
					_ = s.videoRepo.AddFavoriteSetCache(bgCtx, videoID, currentUserID, 24*time.Hour)
				}(detachedCtx)
			}
		} else {
			isFavor, _ = s.videoRepo.IsMemberFavoriteSetCache(ctx, videoID, currentUserID)
		}
	}

	// =========================================================================
	// 🎯 第五方阵：🧱 核心组装，高级数据视窗对象 (VO) 正式出闸发货
	// =========================================================================
	vo := &response.VideoVO{
		ID:            video.ID,
		Title:         video.Title,
		VideoUrl:      video.VideoUrl,
		CoverUrl:      video.CoverUrl,
		Duration:      video.Duration,
		LikeCount:     cacheLikeCount,
		FavoriteCount: cacheFavoriteCount,
		CreatedAt:     video.CreatedAt,
		Tags:          video.Tags,
		Author: response.AuthorInfo{
			ID:        author.ID,
			Username:  author.Username,
			Avatar:    author.HeadImg,
			Signature: author.Signature,
		},
		IsLike:     isLike,
		IsFavorite: isFavor,
		TargetType: "video",
	}

	return vo, nil
}

// 💡 1. 请同步将 VideoService 接口中的签名修改为：
// GetUserLikeVideoListService(ctx context.Context, targetUserID int64, currentUserID int64) ([]response.VideoVO, error)

func (s *videoService) GetUserLikeVideoListService(ctx context.Context, targetUserID int64, currentUserID int64) ([]response.VideoVO, error) {
	// 1. 传唤点赞仓库，斩获目标用户点赞的所有视频 ID 数组
	userLikeIDs, err := s.likeRepo.GetLikedVideoIDs(ctx, targetUserID)
	if err != nil {
		global.LogCtx(ctx).Errorw("❌ [Repository] 未能成功调取用户点赞视频的ID大账本", "userID", targetUserID, "Error", err)
		return nil, err
	}
	if len(userLikeIDs) == 0 {
		return []response.VideoVO{}, nil
	}

	// 2. 传唤视频仓库，利用 SQL 的 IN 语句批量收网视频 PoJo 实体
	// 💡 注意：对齐我们之前在 VideoRepository 里面统一的批量方法名 GetVideosByIDs
	pojoVideos, err := s.videoRepo.GetVideoByIDList(ctx, userLikeIDs)
	if err != nil {
		global.LogCtx(ctx).Errorw("❌ [MySQL] 通过ID数组批量拉取视频实体发生大出轨", "Error", err)
		return nil, err
	}
	if len(pojoVideos) == 0 {
		return []response.VideoVO{}, nil
	}

	// =========================================================================
	// 🎯 核心升级【消灭 N+1】：在内存中动态构筑当前登录用户的“红心与星星”状态矩阵
	// =========================================================================
	likedVideoMap := make(map[int64]bool)
	favoritedVideoMap := make(map[int64]bool)

	if currentUserID > 0 && len(pojoVideos) > 0 {
		// A. 判定点赞状态 (IsLike)
		if currentUserID == targetUserID {
			// 如果是看自己的喜欢列表，那这批视频对自己来说必然全是点过赞的，直接全填 true
			for _, v := range pojoVideos {
				likedVideoMap[v.ID] = true
			}
		} else {
			// 如果是看别人的喜欢列表，必须高并发一枪查出当前登录用户对这批视频哪些点过赞
			likedIDs, _ := s.videoRepo.BatchGetLikedVideoIDs(ctx, currentUserID, userLikeIDs)
			for _, id := range likedIDs {
				likedVideoMap[id] = true
			}
		}

		// B. 判定收藏状态 (IsFavorite)
		// 无论看谁的喜欢列表，当前用户是否收藏了这批视频，都必须去批量账本里查一枪，消灭熄灭 Bug！
		favoritedIDs, _ := s.videoRepo.BatchGetFavoriteVideoIDs(ctx, currentUserID, userLikeIDs)
		for _, id := range favoritedIDs {
			favoritedVideoMap[id] = true
		}
	}

	// 3. 内存高级多路拼装流水线开始穿针引线
	videoVOs := make([]response.VideoVO, 0, len(pojoVideos))
	for _, video := range pojoVideos {
		// 聚合作者空间社交名片（优先从 Redis 动态哈希抠取）
		var author pojo.User
		userKey := fmt.Sprintf("UserProfile:%d", video.AuthorID)
		_ = global.GVA_REDIS.HGetAll(ctx, userKey).Scan(&author)
		if author.ID == 0 {
			author, _ = s.userRepo.QueryByID(ctx, int(video.AuthorID))
		}

		// 抓取高并发行锁保护下的最新点赞/收藏计数安全数字
		likeCountKey := fmt.Sprintf("Like:Count:video:%d", video.ID)
		likeCount, _ := global.GVA_REDIS.Get(ctx, likeCountKey).Int64()
		if likeCount == 0 {
			likeCount = video.LikeCount
		}

		favoriteCountKey := fmt.Sprintf("Favorite:Count:video:%d", video.ID)
		favCount, _ := global.GVA_REDIS.Get(ctx, favoriteCountKey).Int64()
		if favCount == 0 {
			favCount = video.FavoriteCount
		}

		// 装配 Vue3 极度渴望的奢华 VO 对象
		vo := response.VideoVO{
			ID:            video.ID,
			Title:         video.Title,
			VideoUrl:      video.VideoUrl,
			CoverUrl:      video.CoverUrl,
			Duration:      video.Duration,
			LikeCount:     likeCount,
			FavoriteCount: favCount,
			CreatedAt:     video.CreatedAt,
			Tags:          video.Tags,
			Author: response.AuthorInfo{
				ID:        author.ID,
				Username:  author.Username,
				Avatar:    author.HeadImg,
				Signature: author.Signature,
			},
			IsLike:     likedVideoMap[video.ID],     // 💡 动态并网：看别人主页时，精准回显我自己到底有没有点赞
			IsFavorite: favoritedVideoMap[video.ID], // 💡 动态并网：精准回显我自己到底有没有收藏过它
			TargetType: "video",
		}
		videoVOs = append(videoVOs, vo)
	}

	return videoVOs, nil
}
func (s *videoService) GetUserTotalWorkCountService(ctx context.Context, authorID int64) (int64, error) {
	// A. 拦截：优先从 Redis 缓存中抠数字
	count, hit, _ := s.videoRepo.GetVideoCountCache(ctx, authorID)
	if hit {
		return count, nil
	}

	// B. 穿透/未击中：回源 MySQL 查真实总数
	count, err := s.videoRepo.GetVideoCountByAuthorID(ctx, authorID)
	if err != nil {
		global.LogCtx(ctx).Errorw("❌ [MySQL] 回源查询用户作品总数失败", "authorID", authorID, "err", err)
		return 0, err
	}

	// C. 赛博分流：开启异步协程回填 Redis，绝不卡住主请求线程
	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, uid int64, cnt int64) {
		bgCtx, cancel := context.WithTimeout(traceCtx, 3*time.Second)
		defer cancel()
		_ = s.videoRepo.SetVideoCountCache(bgCtx, uid, cnt)
	}(detachedCtx, authorID, count)

	return count, nil
}

// GetUserFavoriteVideoListService 💡 1. 记得在 VideoService 接口中把签名同步修改为：
func (s *videoService) GetUserFavoriteVideoListService(ctx context.Context, targetUserID int64, currentUserID int64) ([]response.VideoVO, error) {
	// 1. 传唤收藏仓库，斩获目标用户收藏的所有视频 ID 数组
	userFavoriteIDs, err := s.favRepo.GetFavoriteVideoIDs(ctx, targetUserID)
	if err != nil {
		global.LogCtx(ctx).Errorw("❌ [Repository] 未能成功调取用户收藏视频的ID大账本", "userID", targetUserID, "Error", err)
		return nil, err
	}
	if len(userFavoriteIDs) == 0 {
		return []response.VideoVO{}, nil
	}

	// 2. 传唤视频仓库，利用 SQL 的 IN 语句单枪击穿，批量收网视频 PoJo 实体
	pojoVideos, err := s.videoRepo.GetVideoByIDList(ctx, userFavoriteIDs)
	if err != nil {
		global.LogCtx(ctx).Errorw("❌ [MySQL] 通过ID数组批量拉取视频实体发生大出轨", "Error", err)
		return nil, err
	}
	if len(pojoVideos) == 0 {
		return []response.VideoVO{}, nil
	}

	// =========================================================================
	// 🎯 核心升级【消灭 N+1】：批量捞取当前登录用户对这批收藏视频的点赞全景图
	// =========================================================================
	likedVideoMap := make(map[int64]bool)
	if currentUserID > 0 && len(pojoVideos) > 0 {
		// 传唤点赞仓库，一枪查出当前登录用户对这批视频哪些点过赞（ status = 1 ）
		// 这里复用了我们之前在 VideoRepository/LikeRepository 声明的批量账本Pluck方法
		likedIDs, err := s.videoRepo.BatchGetLikedVideoIDs(ctx, currentUserID, userFavoriteIDs)
		if err == nil {
			for _, id := range likedIDs {
				likedVideoMap[id] = true // 扔进内存 O(1) 哈希表，等待焊接
			}
		}
	}

	// 3. 内存高级多路拼装流水线开始穿针引线
	videoVOs := make([]response.VideoVO, 0, len(pojoVideos))
	for _, video := range pojoVideos {
		// 聚合作者空间社交名片（优先从 Redis 动态哈希抠取）
		var author pojo.User
		userKey := fmt.Sprintf("UserProfile:%d", video.AuthorID)
		_ = global.GVA_REDIS.HGetAll(ctx, userKey).Scan(&author)
		if author.ID == 0 {
			author, _ = s.userRepo.QueryByID(ctx, int(video.AuthorID))
		}

		// 抓取高并发行锁保护下的最新点赞/收藏计数安全数字
		likeCountKey := fmt.Sprintf("Like:Count:video:%d", video.ID)
		likeCount, _ := global.GVA_REDIS.Get(ctx, likeCountKey).Int64()
		if likeCount == 0 {
			likeCount = video.LikeCount
		}

		favoriteCountKey := fmt.Sprintf("Favorite:Count:video:%d", video.ID)
		favCount, _ := global.GVA_REDIS.Get(ctx, favoriteCountKey).Int64()
		if favCount == 0 {
			favCount = video.FavoriteCount
		}

		// 装配 Vue3 极度渴望的奢华 VO 对象
		vo := response.VideoVO{
			ID:            video.ID,
			Title:         video.Title,
			VideoUrl:      video.VideoUrl,
			CoverUrl:      video.CoverUrl,
			Duration:      video.Duration,
			LikeCount:     likeCount,
			FavoriteCount: favCount,
			CreatedAt:     video.CreatedAt,
			Tags:          video.Tags,
			Author: response.AuthorInfo{
				ID:        author.ID,
				Username:  author.Username,
				Avatar:    author.HeadImg,
				Signature: author.Signature,
			},
			IsLike:     likedVideoMap[video.ID], // 💡 动态并网：从批量点赞账本里抠，点过赞就红，没点过就灰！
			IsFavorite: true,                    // 💡 铁律锁定：既然从收藏列表里捞出来的，收藏星星100%全亮！
			TargetType: "video",
		}
		videoVOs = append(videoVOs, vo)
	}

	return videoVOs, nil
}
