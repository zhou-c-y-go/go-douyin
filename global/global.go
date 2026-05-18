package global

import (
	"Go_Project/setting"
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
