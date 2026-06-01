package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"context"
)

type CommentRepository struct{}

// CreateComment ── 发表评论/回复
func (r *CommentRepository) CreateComment(ctx context.Context, comment *pojo.Comment) error {
	err := global.GVA_DB.WithContext(ctx).Create(comment).Error
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [DB] 评论写入失败: %v", err)
		return err
	}
	return nil
}

// GetCommentsByVideoID ── 树状递归的物化路径一键拉取
func (r *CommentRepository) GetCommentsByVideoID(ctx context.Context, videoID int64) ([]pojo.Comment, error) {
	var comments []pojo.Comment

	/*
	   因为在设计时采用了 Path 字段（如 "1/", "1/3/", "1/4/9/"），
	   在 MySQL 中，对 varchar 类型的 Path 执行 ASC（正序）排列时，
	   字典序会精妙地自动把“子评论”死死贴在对应的“父评论”正下方！
	   这样前端拿到一个干净的平铺切片列表后，直接顺序渲染就是完美的树状嵌套结构，省去了几十行的递归算力！
	*/
	err := global.GVA_DB.WithContext(ctx).
		Where("video_id = ?", videoID).
		Order("path ASC, created_at ASC"). // 路径正序，先评的在上面
		Find(&comments).Error

	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [DB] 一键拉取视频 [%d] 的物化路径评论树失败: %v", videoID, err)
		return nil, err
	}
	return comments, nil
}
