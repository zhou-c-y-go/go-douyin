package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

// CommentRepository 评论领域持久化标准接口
type CommentRepository interface {
	CreateComment(ctx context.Context, comment *pojo.Comment) error
	GetCommentByID(ctx context.Context, id int64) (*pojo.Comment, error)
	GetCommentsByVideoID(ctx context.Context, videoID int64) ([]pojo.Comment, error)
	UpdateCommentPath(ctx context.Context, id int64, path string) error

	// Cache 缓存自治防线
	GetCommentsCache(ctx context.Context, videoID int64) ([]pojo.Comment, bool, error)
	SetCommentsCache(ctx context.Context, videoID int64, comments []pojo.Comment) error
	DelCommentsCache(ctx context.Context, videoID int64) error

	// 评论点赞全景透传
	GetLikedCommentMap(ctx context.Context, userID int64) (map[int64]bool, error)
	BatchGetLikeCounts(ctx context.Context, commentIDs []int64) (map[int64]int64, error)
}

type commentRepository struct{}

func NewCommentRepository() CommentRepository {
	return &commentRepository{}
}

func (r *commentRepository) CreateComment(ctx context.Context, comment *pojo.Comment) error {
	return global.GVA_DB.WithContext(ctx).Create(comment).Error
}

func (r *commentRepository) GetCommentByID(ctx context.Context, id int64) (*pojo.Comment, error) {
	var comment pojo.Comment
	err := global.GVA_DB.WithContext(ctx).Where("id = ?", id).First(&comment).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *commentRepository) GetCommentsByVideoID(ctx context.Context, videoID int64) ([]pojo.Comment, error) {
	var comments []pojo.Comment
	// 路径正序字典排列，巧妙让子评论死死贴在父评论下方
	err := global.GVA_DB.WithContext(ctx).
		Where("video_id = ?", videoID).
		Order("path ASC, created_at ASC").
		Find(&comments).Error
	return comments, err
}

func (r *commentRepository) UpdateCommentPath(ctx context.Context, id int64, path string) error {
	return global.GVA_DB.WithContext(ctx).Model(&pojo.Comment{}).Where("id = ?", id).Update("path", path).Error
}

func (r *commentRepository) GetCommentsCache(ctx context.Context, videoID int64) ([]pojo.Comment, bool, error) {
	commentListKey := fmt.Sprintf("Comment:List:%d", videoID)
	cacheData, err := global.GVA_REDIS.Get(ctx, commentListKey).Result()
	if err != nil || cacheData == "" {
		return nil, false, err
	}
	var pojoComments []pojo.Comment
	_ = json.Unmarshal([]byte(cacheData), &pojoComments)
	return pojoComments, true, nil
}

func (r *commentRepository) SetCommentsCache(ctx context.Context, videoID int64, comments []pojo.Comment) error {
	commentListKey := fmt.Sprintf("Comment:List:%d", videoID)
	jsonData, _ := json.Marshal(comments)
	return global.GVA_REDIS.Set(ctx, commentListKey, jsonData, time.Hour*24).Err()
}

func (r *commentRepository) DelCommentsCache(ctx context.Context, videoID int64) error {
	commentListKey := fmt.Sprintf("Comment:List:%d", videoID)
	return global.GVA_REDIS.Del(ctx, commentListKey).Err()
}

func (r *commentRepository) GetLikedCommentMap(ctx context.Context, userID int64) (map[int64]bool, error) {
	likedCommentMap := make(map[int64]bool)
	userLikeKey := fmt.Sprintf("User:Like:Comments:%d", userID)

	exists, existErr := global.GVA_REDIS.Exists(ctx, userLikeKey).Result()
	if existErr == nil && exists > 0 {
		likedIDsStr, _ := global.GVA_REDIS.SMembers(ctx, userLikeKey).Result()
		for _, idStr := range likedIDsStr {
			if id, parseErr := strconv.ParseInt(idStr, 10, 64); parseErr == nil {
				likedCommentMap[id] = true
			}
		}
		return likedCommentMap, nil
	}

	var likedIDs []int64
	err := global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
		Where("user_id = ? AND target_type = ? AND status = 1", userID, "comment").
		Pluck("target_id", &likedIDs).Error
	if err != nil {
		return likedCommentMap, err
	}

	if len(likedIDs) > 0 {
		interfaces := make([]interface{}, len(likedIDs))
		for i, id := range likedIDs {
			interfaces[i] = id
			likedCommentMap[id] = true
		}
		global.GVA_REDIS.SAdd(ctx, userLikeKey, interfaces...)
		global.GVA_REDIS.Expire(ctx, userLikeKey, 24*time.Hour)
	}
	return likedCommentMap, nil
}

func (r *commentRepository) BatchGetLikeCounts(ctx context.Context, commentIDs []int64) (map[int64]int64, error) {
	pipe := global.GVA_REDIS.Pipeline()
	countCmds := make(map[int64]*redis.StringCmd)
	for _, id := range commentIDs {
		countKey := fmt.Sprintf("Like:Count:comment:%d", id)
		countCmds[id] = pipe.Get(ctx, countKey)
	}
	_, _ = pipe.Exec(ctx)

	result := make(map[int64]int64)
	for id, cmd := range countCmds {
		if cnt, err := cmd.Int64(); err == nil {
			result[id] = cnt
		}
	}
	return result, nil
}
