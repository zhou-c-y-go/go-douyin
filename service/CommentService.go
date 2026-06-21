package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/model/response"
	repo "Go_Project/common/repository"
	"Go_Project/global"
	"context"
	"errors"
	"fmt"
	"time"
)

// CommentService 评论模块业务标准接口
type CommentService interface {
	GetVideoCommentTreeService(ctx context.Context, videoID int64, currentUserID int64) ([]*response.CommentVO, error)
	PublishCommentService(ctx context.Context, videoID int64, userID int64, content string, replyToID int64) error
}

type commentService struct {
	commentRepo repo.CommentRepository
	userRepo    repo.UserRepository // 强力并网：跨模块依赖注入
}

func NewCommentService(cr repo.CommentRepository, ur repo.UserRepository) CommentService {
	return &commentService{
		commentRepo: cr,
		userRepo:    ur,
	}
}

func (s *commentService) GetVideoCommentTreeService(ctx context.Context, videoID int64, currentUserID int64) ([]*response.CommentVO, error) {
	var pojoComments []pojo.Comment
	var err error
	var hit bool

	// 1. 干净的缓存拦截
	pojoComments, hit, err = s.commentRepo.GetCommentsCache(ctx, videoID)
	if !hit {
		pojoComments, err = s.commentRepo.GetCommentsByVideoID(ctx, videoID)
		if err != nil {
			global.LogCtx(ctx).Errorw("从MySQL中拉取评论树失败", "Error", err)
			return nil, err
		}
		if len(pojoComments) == 0 {
			return []*response.CommentVO{}, nil
		}
		_ = s.commentRepo.SetCommentsCache(ctx, videoID, pojoComments)
	}

	// 2. 收集用户ID并进行跨模块批量 Pluck 捞取
	userIDs := make([]int64, 0)
	userMapUnique := make(map[int64]bool)
	for _, c := range pojoComments {
		if !userMapUnique[c.UserID] {
			userMapUnique[c.UserID] = true
			userIDs = append(userIDs, c.UserID)
		}
	}
	userCardMap, _ := s.userRepo.BatchGetUserCardMap(ctx, userIDs)

	// 3. 批量拦截当前登录用户的点赞账本
	likedCommentMap := make(map[int64]bool)
	if currentUserID > 0 {
		likedCommentMap, _ = s.commentRepo.GetLikedCommentMap(ctx, currentUserID)
	}

	// 4. Pipeline 并网获取点赞总数
	commentIDs := make([]int64, len(pojoComments))
	for i, pc := range pojoComments {
		commentIDs[i] = pc.ID
	}
	likeCountsMap, _ := s.commentRepo.BatchGetLikeCounts(ctx, commentIDs)

	// 5. 终极两轮焊接算法成树
	voMap := make(map[int64]*response.CommentVO)
	rootComments := make([]*response.CommentVO, 0)

	// 第一轮：构建 VO 内存映射
	for _, pc := range pojoComments {
		realtimeLikeCount := pc.LikeCount
		if cnt, exists := likeCountsMap[pc.ID]; exists {
			realtimeLikeCount = cnt
		}
		vo := &response.CommentVO{
			ID:         pc.ID,
			VideoID:    pc.VideoID,
			Content:    pc.Content,
			Path:       pc.Path,
			ReplyToID:  pc.ReplyToID,
			CreatedAt:  pc.CreatedAt,
			User:       userCardMap[pc.UserID],
			Children:   make([]*response.CommentVO, 0),
			TargetType: "comment",
			LikeCount:  realtimeLikeCount,
			IsLiked:    likedCommentMap[pc.ID],
		}
		voMap[vo.ID] = vo
	}

	// 第二轮：顺序挂载子节点
	for _, pc := range pojoComments {
		currentVO := voMap[pc.ID]
		if pc.ReplyToID == 0 {
			rootComments = append(rootComments, currentVO)
		} else {
			if parentVO, exists := voMap[pc.ReplyToID]; exists {
				parentVO.Children = append(parentVO.Children, currentVO)
			}
		}
	}
	return rootComments, nil
}

func (s *commentService) PublishCommentService(ctx context.Context, videoID int64, userID int64, content string, replyToID int64) error {
	newComment := &pojo.Comment{
		VideoID:   videoID,
		UserID:    userID,
		Content:   content,
		ReplyToID: replyToID,
	}

	// 第一次落盘拿到自增 ID
	if err := s.commentRepo.CreateComment(ctx, newComment); err != nil {
		return errors.New("评论发射失败，数据库接收端流产")
	}

	// 物化路径计算
	var newPath string
	if replyToID == 0 {
		newPath = fmt.Sprintf("%d/", newComment.ID)
	} else {
		parentComment, err := s.commentRepo.GetCommentByID(ctx, replyToID)
		if err != nil {
			newPath = fmt.Sprintf("%d/", newComment.ID)
		} else {
			newPath = fmt.Sprintf("%s%d/", parentComment.Path, newComment.ID)
		}
	}

	// 第二次落盘更新 Path 字段
	if err := s.commentRepo.UpdateCommentPath(ctx, newComment.ID, newPath); err != nil {
		return errors.New("评论树路径装配发生异常")
	}

	// 数据一致性淘汰防御
	_ = s.commentRepo.DelCommentsCache(ctx, videoID)

	// 继承 TraceID 的高并发后台异步预热协程
	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, vID int64) {
		bgCtx, cancel := context.WithTimeout(traceCtx, 5*time.Second)
		defer cancel()
		freshComments, freshErr := s.commentRepo.GetCommentsByVideoID(bgCtx, vID)
		if freshErr == nil && len(freshComments) > 0 {
			_ = s.commentRepo.SetCommentsCache(bgCtx, vID, freshComments)
		}
	}(detachedCtx, videoID)

	return nil
}
