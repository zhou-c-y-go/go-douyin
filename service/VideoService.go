package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"context"
)

type VideoService struct {
}

func (s *VideoService) GetFeedListMiddle(ctx context.Context) ([]pojo.Video, error) {
	// 💡 自动带上追踪身份证号
	global.LogCtx(ctx).Info("开始拉取首页公共视频流数据")
	var list []pojo.Video
	err := global.GVA_DB.Order("id DESC").Limit(10).Find(&list).Error
	if err != nil {
		global.LogCtx(ctx).Errorw("拉取视频流数据库大翻车", "err", err)
		return nil, err
	}
	return list, nil
}
