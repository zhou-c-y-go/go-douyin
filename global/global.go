package global

import (
	"Go_Project/setting"
	"context"
	"github.com/minio/minio-go"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// 全局变量管理文件
var (
	Logger                   *zap.Logger
	SugaredLogger            *zap.SugaredLogger
	GLOB_CONFIG              *setting.Configs
	GVA_VP                   *viper.Viper
	GVA_DB                   *gorm.DB
	GVA_REDIS                *redis.Client
	MinioClient              *minio.Client
	GLOB_Concurrency_Control = &singleflight.Group{}
)

func LogCtx(ctx context.Context) *zap.SugaredLogger {
	traceId := GetTraceId(ctx)
	if traceId != "" {
		// 2. 如果存在，利用 Zap 的 With 机制，给这条日志强行绑上身份证号字段
		return SugaredLogger.With("traceId", traceId)
	}

	// 3. 如果没拿到，返回普通日志对象
	return SugaredLogger
}
