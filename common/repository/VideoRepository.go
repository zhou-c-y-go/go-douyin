package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"context"
	"fmt"
	"time"
)

// VideoRepository 视频领域持久化标准接口
type VideoRepository interface {
	CreateVideo(ctx context.Context, video *pojo.Video) error
	GetVideosForFeed(ctx context.Context, latestTime time.Time, limit int) ([]pojo.Video, error)
	GetVideosByAuthorID(ctx context.Context, authorID int64) ([]pojo.Video, error)
	GetVideoByID(ctx context.Context, id int64) (*pojo.Video, error)
	UpdateDuration(ctx context.Context, videoID int64, duration int64) error

	// 社交关联透传查询
	BatchGetLikedVideoIDs(ctx context.Context, userID int64, videoIDs []int64) ([]int64, error)
	CheckLikeRecordExists(ctx context.Context, userID, videoID int64) (bool, error)
	CheckFavoriteRecordExists(ctx context.Context, userID, videoID int64) (bool, error)

	// Cache 缓存自治防线
	GetGlobalFeedCache(ctx context.Context) (string, error)
	SetGlobalFeedCache(ctx context.Context, data string, ttl time.Duration) error
	DelGlobalFeedCache(ctx context.Context) error
	AddVideoToPoolCache(ctx context.Context, videoID int64) error
	DelUserVideoListCache(ctx context.Context, authorID int64) error
	GetUserVideoListCache(ctx context.Context, authorID int64) (string, error)
	SetUserVideoListCache(ctx context.Context, authorID int64, data string, ttl time.Duration) error

	GetVideoLikeCountCache(ctx context.Context, videoID int64) (int64, error)
	SetVideoLikeCountCache(ctx context.Context, videoID int64, count int64, ttl time.Duration) error
	GetVideoFavoriteCountCache(ctx context.Context, videoID int64) (int64, error)
	SetVideoFavoriteCountCache(ctx context.Context, videoID int64, count int64, ttl time.Duration) error

	ExistsLikeSetCache(ctx context.Context, videoID int64) (int64, error)
	IsMemberLikeSetCache(ctx context.Context, videoID int64, userID int64) (bool, error)
	AddLikeSetCache(ctx context.Context, videoID int64, userID int64, ttl time.Duration) error

	ExistsFavoriteSetCache(ctx context.Context, videoID int64) (int64, error)
	IsMemberFavoriteSetCache(ctx context.Context, videoID int64, userID int64) (bool, error)
	AddFavoriteSetCache(ctx context.Context, videoID int64, userID int64, ttl time.Duration) error
}

type videoRepository struct{}

func NewVideoRepository() VideoRepository {
	return &videoRepository{}
}

func (r *videoRepository) CreateVideo(ctx context.Context, video *pojo.Video) error {
	return global.GVA_DB.WithContext(ctx).Create(video).Error
}

func (r *videoRepository) GetVideosForFeed(ctx context.Context, latestTime time.Time, limit int) ([]pojo.Video, error) {
	var videos []pojo.Video
	err := global.GVA_DB.WithContext(ctx).Where("created_at < ?", latestTime).Order("created_at DESC").Limit(limit).Find(&videos).Error
	return videos, err
}

func (r *videoRepository) GetVideosByAuthorID(ctx context.Context, authorID int64) ([]pojo.Video, error) {
	var videos []pojo.Video
	err := global.GVA_DB.WithContext(ctx).Where("author_id = ?", authorID).Order("created_at DESC").Find(&videos).Error
	return videos, err
}

func (r *videoRepository) GetVideoByID(ctx context.Context, id int64) (*pojo.Video, error) {
	var video pojo.Video
	err := global.GVA_DB.WithContext(ctx).Where("id = ?", id).First(&video).Error
	if err != nil {
		return nil, err
	}
	return &video, nil
}

func (r *videoRepository) UpdateDuration(ctx context.Context, videoID int64, duration int64) error {
	return global.GVA_DB.WithContext(ctx).Model(&pojo.Video{}).Where("id = ?", videoID).Update("duration", duration).Error
}

func (r *videoRepository) BatchGetLikedVideoIDs(ctx context.Context, userID int64, videoIDs []int64) ([]int64, error) {
	var likedIDs []int64
	err := global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
		Where("user_id = ? AND target_type = ? AND target_id IN ? AND status = 1", userID, "video", videoIDs).
		Pluck("target_id", &likedIDs).Error
	return likedIDs, err
}

func (r *videoRepository) CheckLikeRecordExists(ctx context.Context, userID, videoID int64) (bool, error) {
	var count int64
	err := global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
		Where("user_id = ? AND target_id = ? AND target_type = ? AND status = 1", userID, videoID, "video").
		Count(&count).Error
	return count > 0, err
}

func (r *videoRepository) CheckFavoriteRecordExists(ctx context.Context, userID, videoID int64) (bool, error) {
	var count int64
	err := global.GVA_DB.WithContext(ctx).Model(&pojo.FavoriteRecord{}).
		Where("user_id = ? AND target_id = ? AND target_type = ? AND status = 1", userID, videoID, "video").
		Count(&count).Error
	return count > 0, err
}

// ---- 下方为完全剥离自 Service 层的 Redis 缓存接口实现 ----

func (r *videoRepository) GetGlobalFeedCache(ctx context.Context) (string, error) {
	return global.GVA_REDIS.Get(ctx, "GlobalFeedCache").Result()
}

func (r *videoRepository) SetGlobalFeedCache(ctx context.Context, data string, ttl time.Duration) error {
	return global.GVA_REDIS.Set(ctx, "GlobalFeedCache", data, ttl).Err()
}

func (r *videoRepository) DelGlobalFeedCache(ctx context.Context) error {
	return global.GVA_REDIS.Del(ctx, "GlobalFeedCache").Err()
}

func (r *videoRepository) AddVideoToPoolCache(ctx context.Context, videoID int64) error {
	return global.GVA_REDIS.SAdd(ctx, "GlobalVideoPool", videoID).Err()
}

func (r *videoRepository) DelUserVideoListCache(ctx context.Context, authorID int64) error {
	return global.GVA_REDIS.Del(ctx, fmt.Sprintf("UserVideoList:%d", authorID)).Err()
}

func (r *videoRepository) GetUserVideoListCache(ctx context.Context, authorID int64) (string, error) {
	return global.GVA_REDIS.Get(ctx, fmt.Sprintf("UserVideoList:%d", authorID)).Result()
}

func (r *videoRepository) SetUserVideoListCache(ctx context.Context, authorID int64, data string, ttl time.Duration) error {
	return global.GVA_REDIS.Set(ctx, fmt.Sprintf("UserVideoList:%d", authorID), data, ttl).Err()
}

func (r *videoRepository) GetVideoLikeCountCache(ctx context.Context, videoID int64) (int64, error) {
	return global.GVA_REDIS.Get(ctx, fmt.Sprintf("Like:Count:video:%d", videoID)).Int64()
}

func (r *videoRepository) SetVideoLikeCountCache(ctx context.Context, videoID int64, count int64, ttl time.Duration) error {
	return global.GVA_REDIS.Set(ctx, fmt.Sprintf("Like:Count:video:%d", videoID), count, ttl).Err()
}

func (r *videoRepository) GetVideoFavoriteCountCache(ctx context.Context, videoID int64) (int64, error) {
	return global.GVA_REDIS.Get(ctx, fmt.Sprintf("Favorite:Count:video:%d", videoID)).Int64()
}

func (r *videoRepository) SetVideoFavoriteCountCache(ctx context.Context, videoID int64, count int64, ttl time.Duration) error {
	return global.GVA_REDIS.Set(ctx, fmt.Sprintf("Favorite:Count:video:%d", videoID), count, ttl).Err()
}

func (r *videoRepository) ExistsLikeSetCache(ctx context.Context, videoID int64) (int64, error) {
	return global.GVA_REDIS.Exists(ctx, fmt.Sprintf("Like:Set:video:%d", videoID)).Result()
}

func (r *videoRepository) IsMemberLikeSetCache(ctx context.Context, videoID int64, userID int64) (bool, error) {
	return global.GVA_REDIS.SIsMember(ctx, fmt.Sprintf("Like:Set:video:%d", videoID), userID).Result()
}

func (r *videoRepository) AddLikeSetCache(ctx context.Context, videoID int64, userID int64, ttl time.Duration) error {
	key := fmt.Sprintf("Like:Set:video:%d", videoID)
	if err := global.GVA_REDIS.SAdd(ctx, key, userID).Err(); err != nil {
		return err
	}
	return global.GVA_REDIS.Expire(ctx, key, ttl).Err()
}

func (r *videoRepository) ExistsFavoriteSetCache(ctx context.Context, videoID int64) (int64, error) {
	return global.GVA_REDIS.Exists(ctx, fmt.Sprintf("Favorite:Set:video:%d", videoID)).Result()
}

func (r *videoRepository) IsMemberFavoriteSetCache(ctx context.Context, videoID int64, userID int64) (bool, error) {
	return global.GVA_REDIS.SIsMember(ctx, fmt.Sprintf("Favorite:Set:video:%d", videoID), userID).Result()
}

func (r *videoRepository) AddFavoriteSetCache(ctx context.Context, videoID int64, userID int64, ttl time.Duration) error {
	key := fmt.Sprintf("Favorite:Set:video:%d", videoID)
	if err := global.GVA_REDIS.SAdd(ctx, key, userID).Err(); err != nil {
		return err
	}
	return global.GVA_REDIS.Expire(ctx, key, ttl).Err()
}
