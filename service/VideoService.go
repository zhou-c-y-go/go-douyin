package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/model/response"
	repo "Go_Project/common/repository"
	"Go_Project/utils"
	"context"
	"encoding/json"
	"errors"
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
}

type videoService struct {
	videoRepo repo.VideoRepository
	userRepo  repo.UserRepository // 引入以支持内存多表并网聚合
}

func NewVideoService(vr repo.VideoRepository, ur repo.UserRepository) VideoService {
	return &videoService{videoRepo: vr, userRepo: ur}
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
	}(detachedCtx, videoEntity.ID)
	return nil
}

func (s *videoService) GetUserVideoListService(ctx context.Context, targetUserID int64) ([]response.VideoVO, error) {
	cachedData, err := s.videoRepo.GetUserVideoListCache(ctx, targetUserID)
	if err == nil && cachedData != "" {
		var cachedVOs []response.VideoVO
		if json.Unmarshal([]byte(cachedData), &cachedVOs) == nil {
			return cachedVOs, nil
		}
	}

	pojoVideos, err := s.videoRepo.GetVideosByAuthorID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

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

	videoVOs := make([]response.VideoVO, 0, len(pojoVideos))
	for _, v := range pojoVideos {
		likeCount, err := s.videoRepo.GetVideoLikeCountCache(ctx, v.ID)
		if err != nil {
			likeCount = v.LikeCount
		}
		favCount, err := s.videoRepo.GetVideoFavoriteCountCache(ctx, v.ID)
		if err != nil {
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
		_ = s.videoRepo.SetUserVideoListCache(ctx, targetUserID, string(jsonData), 10*time.Minute)
	}
	return videoVOs, nil
}

func (s *videoService) GetVideoDetailService(ctx context.Context, videoID int64, currentUserID int64) (*response.VideoVO, error) {
	video, err := s.videoRepo.GetVideoByID(ctx, videoID)
	if err != nil {
		return nil, errors.New("视频可能已迷失")
	}

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

	cacheLikeCount, err := s.videoRepo.GetVideoLikeCountCache(ctx, videoID)
	if err != nil {
		cacheLikeCount = video.LikeCount
		_ = s.videoRepo.SetVideoLikeCountCache(ctx, videoID, cacheLikeCount, 24*time.Hour)
	}
	cacheFavoriteCount, err := s.videoRepo.GetVideoFavoriteCountCache(ctx, videoID)
	if err != nil {
		cacheFavoriteCount = video.FavoriteCount
		_ = s.videoRepo.SetVideoFavoriteCountCache(ctx, videoID, cacheFavoriteCount, 24*time.Hour)
	}

	var isLike, isFavor bool
	if currentUserID > 0 {
		keyExists, err := s.videoRepo.ExistsLikeSetCache(ctx, videoID)
		if err != nil || keyExists == 0 {
			isLike, _ = s.videoRepo.CheckLikeRecordExists(ctx, currentUserID, videoID)
			if isLike {
				go func() {
					bgCtx := context.Background()
					_ = s.videoRepo.AddLikeSetCache(bgCtx, videoID, currentUserID, 24*time.Hour)
				}()
			}
		} else {
			isLike, _ = s.videoRepo.IsMemberLikeSetCache(ctx, videoID, currentUserID)
		}

		keyExists, err = s.videoRepo.ExistsFavoriteSetCache(ctx, videoID)
		if err != nil || keyExists == 0 {
			isFavor, _ = s.videoRepo.CheckFavoriteRecordExists(ctx, currentUserID, videoID)
			if isFavor {
				go func() {
					bgCtx := context.Background()
					_ = s.videoRepo.AddFavoriteSetCache(bgCtx, videoID, currentUserID, 24*time.Hour)
				}()
			}
		} else {
			isFavor, _ = s.videoRepo.IsMemberFavoriteSetCache(ctx, videoID, currentUserID)
		}
	}

	vo := &response.VideoVO{
		ID: video.ID, Title: video.Title, VideoUrl: video.VideoUrl, CoverUrl: video.CoverUrl,
		Duration: video.Duration, LikeCount: cacheLikeCount, FavoriteCount: cacheFavoriteCount, CreatedAt: video.CreatedAt, Tags: video.Tags,
		Author: response.AuthorInfo{
			ID: author.ID, Username: author.Username, Avatar: author.HeadImg, Signature: author.Signature,
		},
		IsLike: isLike, IsFavorite: isFavor, TargetType: "video",
	}
	return vo, nil
}
