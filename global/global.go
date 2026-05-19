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
	if ctx == nil {
		return SugaredLogger
	}
	// 从 Go 标准上下文中取出 TraceID
	if traceID, ok := ctx.Value("traceId").(string); ok {
		// 💡 利用 Zap 的 With 功能，后续这行日志后面会自动追加 "traceId": "xxxx"
		return SugaredLogger.With("traceId", traceID)
	}
	return SugaredLogger
}
