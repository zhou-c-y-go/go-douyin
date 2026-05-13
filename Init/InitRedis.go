package Init

import (
	"Go_Project/global"
	"context"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func Redis() {
	redisConfig := global.GLOB_CONFIG.Redis
	client := redis.NewClient(
		&redis.Options{
			Addr:     redisConfig.Addr,
			Password: redisConfig.Password,
			DB:       redisConfig.DB,
		},
	)
	pong, err := client.Ping(context.Background()).Result()
	if err != nil {
		global.SugaredLogger.Error("redis connect ping failed, err:", zap.Error(err))
	} else {
		global.SugaredLogger.Info("redis connect ping response:", zap.String("pong", pong))
		global.GVA_REDIS = client
	}
}
