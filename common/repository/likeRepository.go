package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"context"
)

type LikeRepository struct{}

// GetLikeRecord ── 获取单条点赞历史
func (r *LikeRepository) GetLikeRecord(ctx context.Context, userID, targetID int64, targetType string) (*pojo.LikeRecord, error) {
	var record pojo.LikeRecord
	err := global.GVA_DB.WithContext(ctx).
		Where("user_id = ? AND target_id = ? AND target_type = ?", userID, targetID, targetType).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// CreateLike ── 首次点赞落盘
func (r *LikeRepository) CreateLike(ctx context.Context, like *pojo.LikeRecord) error {
	err := global.GVA_DB.WithContext(ctx).Create(like).Error
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [DB] 点赞明细写入失败: %v", err)
		return err
	}
	return nil
}

// UpdateLikeStatus ── 翻转已有点赞的状态
func (r *LikeRepository) UpdateLikeStatus(ctx context.Context, id int64, status int8) error {
	err := global.GVA_DB.WithContext(ctx).
		Model(&pojo.LikeRecord{}).
		Where("id = ?", id).
		Update("status", status).Error
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [DB] 更新点赞状态失败(ID: %d): %v", id, err)
		return err
	}
	return nil
}
