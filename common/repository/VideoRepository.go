package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"context"
	"time"
)

type VideoRepository struct{}

// CreateVideo ── 建立视频
func (r *VideoRepository) CreateVideo(ctx context.Context, video *pojo.Video) error {
	// 绑定 context 传递 TraceID，利用 GORM 原生 Insert 动作落盘
	err := global.GVA_DB.WithContext(ctx).Create(video).Error
	if err != nil {
		global.LogCtx(ctx).Errorf("[DB] 视频记录写入失败: %v", err)
		return err
	}
	return nil
}

// GetVideosForFeed ── Feed 流核心推荐捞取（基于发布时间降序）
// latestTime: 游标时间戳，防止用户刷到重复视频；limit: 一次下拉限制刷出的条数（如 5条）
func (r *VideoRepository) GetVideosForFeed(ctx context.Context, latestTime time.Time, limit int) ([]pojo.Video, error) {
	var videos []pojo.Video

	// 💡 大厂工业索引优化：利用 created_at 倒序排列，并且强行卡住时间游标
	err := global.GVA_DB.WithContext(ctx).
		Where("created_at < ?", latestTime).
		Order("created_at DESC").
		Limit(limit).
		Find(&videos).Error

	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [DB] 批量捞取 Feed 视频流翻车: %v", err)
		return nil, err
	}
	return videos, nil
}

// GetVideosByAuthorID ── 聚合用户个人主页的“作品”列表
func (r *VideoRepository) GetVideosByAuthorID(ctx context.Context, authorID int64) ([]pojo.Video, error) {
	var videos []pojo.Video

	// 显式命中在 AuthorID 上铺设的底层索引
	err := global.GVA_DB.WithContext(ctx).
		Where("author_id = ?", authorID).
		Order("created_at DESC").
		Find(&videos).Error

	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [DB] 聚合用户 [%d] 的作品列表失败: %v", authorID, err)
		return nil, err
	}
	return videos, nil
}

// GetVideoByID ── 🔍 物理单点捞取：精确锁定单个视频大屏记录
func (r *VideoRepository) GetVideoByID(ctx context.Context, id int64) (*pojo.Video, error) {
	var video pojo.Video
	// 用 First 确保只查一条，自带 Limit 1 极致性能
	err := global.GVA_DB.WithContext(ctx).Where("id = ?", id).First(&video).Error
	if err != nil {
		return nil, err
	}
	return &video, nil
}
