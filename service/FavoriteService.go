package service

import (
	"Go_Project/common/repository"
	"Go_Project/global"
	"context"
	"fmt"
)

type FavorService interface {
	ToggleFavorService(ctx context.Context, userID, targetID int64, targetType string, reqStatus int) (bool, error)
}

type favorService struct {
	favRepo repository.FavoriteRepository
}

func NewFavorService(fr repository.FavoriteRepository) FavorService {
	return &favorService{favRepo: fr} //
}

func (s *favorService) ToggleFavorService(ctx context.Context, userID, targetID int64, targetType string, reqStatus int) (bool, error) {
	// 1. 内存高速研判状态
	isFavor, err := s.favRepo.IsMemberFavoriteSet(ctx, targetType, targetID, userID) //
	if err != nil {
		return false, err //
	}
	var delta int64 = 0             //
	if reqStatus == 1 && !isFavor { //
		delta = 1 //
	} else if reqStatus == 0 && isFavor { //
		delta = -1 //
	}

	if delta == 0 {
		return reqStatus == 1, nil //
	}

	// 2. 内存原子操作：织网与自增计数
	if delta == 1 {
		_ = s.favRepo.AddFavoriteSetAndIncrCount(ctx, targetType, targetID, userID) //
	} else {
		_ = s.favRepo.RemFavoriteSetAndDecrCount(ctx, targetType, targetID, userID) //
	}

	// 3. ⚡【联动并网】：同步原子修改 User:Counters:{uid} 的收藏总数大账本
	counterKey := fmt.Sprintf("User:Counters:%d", userID)
	_ = global.GVA_REDIS.HIncrBy(ctx, counterKey, "favorite_count_total", delta) //

	// =========================================================================
	// 🚀【收藏大动脉升级】：彻底掐断同步查库！留下两层暗号，2ms 闪电发货
	// =========================================================================
	// 暗号一：告诉收藏永动机，哪个视频/文章的【收藏总数】变了
	dirtyToken := fmt.Sprintf("%s:%d", targetType, targetID)
	_ = s.favRepo.MarkFavoriteTargetAsDirty(ctx, dirtyToken)

	// 暗号二：告诉收藏永动机，【谁收藏/取消收藏了谁】，打包丢进流水线 List 队列
	recordPayload := fmt.Sprintf("%d:%d:%s:%d", userID, targetID, targetType, reqStatus)
	_ = s.favRepo.PushFavoriteRecordQueue(ctx, recordPayload)

	return reqStatus == 1, nil //
}
