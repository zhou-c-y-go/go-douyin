package service

import (
	"Go_Project/common/repository"
	"Go_Project/static"
	"Go_Project/utils"
	"context"
	"time"
)

type FavorService interface {
	ToggleFavorService(ctx context.Context, userID, targetID int64, targetType string, reqStatus int) (bool, error)
	GetFavoriteTotalFavoriteCount(ctx context.Context, userID int64) (int64, error)
}

type favorService struct {
	favRepo repository.FavoriteRepository
}

func NewFavorService(fr repository.FavoriteRepository) FavorService {
	return &favorService{favRepo: fr}
}

func (s *favorService) ToggleFavorService(ctx context.Context, userID, targetID int64, targetType string, reqStatus int) (bool, error) {
	isFavor, err := s.favRepo.IsMemberFavoriteSet(ctx, targetType, targetID, userID)
	if err != nil {
		return false, err
	}
	var delta int64 = 0
	if reqStatus == 1 && !isFavor {
		delta = 1
	} else if reqStatus == 0 && isFavor {
		delta = -1
	}

	if delta == 0 {
		return reqStatus == 1, nil
	}
	if delta == 1 {
		_ = s.favRepo.AddFavoriteSetAndIncrCount(ctx, targetType, targetID, userID)
	} else {
		_ = s.favRepo.RemFavoriteSetAndDecrCount(ctx, targetType, targetID, userID)
	}

	detachedCtx := context.WithoutCancel(ctx)
	go func(traceCtx context.Context, uID, tID int64, tType string, status int, d int64) {
		bgCtx, cancel := context.WithTimeout(traceCtx, 5*time.Second)
		defer cancel()
		_ = s.favRepo.SyncFavoriteRecordToDB(bgCtx, uID, tID, tType, status, d)
	}(detachedCtx, userID, targetID, targetType, reqStatus, delta)

	return reqStatus == 1, nil
}

func (s *favorService) GetFavoriteTotalFavoriteCount(ctx context.Context, userID int64) (int64, error) {
	keys := static.GetFavoriteList()
	userFavSetKeys := make([]string, len(keys))
	userFavKeyMap := make(map[string]string)
	for i, tType := range keys {
		userFavSetKeys[i] = utils.GetFavoriteSetKey(tType, userID)
		userFavKeyMap[tType] = userFavSetKeys[i]
	}

	counts, err := s.favRepo.BatchGetFavoriteSetCounts(ctx, userFavSetKeys)
	if err != nil {
		return 0, err
	}
	var total int64 = 0
	allMissing := true
	for _, cnt := range counts {
		if cnt > 0 {
			allMissing = false
		}
		total += cnt
	}

	if allMissing {
		countMap, targetIDsMap, err := s.favRepo.GetRealFavoriteCountAndIDs(ctx, userID, keys)
		if err != nil {
			return 0, err
		}
		// 完美的 Pipeline 批量回填 Redis 缓存宇宙
		_ = s.favRepo.RebuildFavoriteCachePipeline(ctx, userFavKeyMap, targetIDsMap)

		total = 0
		for _, tType := range keys {
			total += countMap[tType]
		}
	}
	return total, nil
}
