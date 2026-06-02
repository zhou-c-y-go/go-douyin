package Init

import (
	"Go_Project/global"
	"Go_Project/utils"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func InitMinio() {
	minioInfo := global.GLOB_CONFIG.Minio
	minioClient, err := minio.New(minioInfo.EndPoint, &minio.Options{
		// 1. 🔑 凭证解耦：使用 NewStaticV4 将密钥对塞入凭证工厂（最后一个参数默认传空字符串 "" 即可）
		Creds: credentials.NewStaticV4(minioInfo.AccessKeyID, minioInfo.SecretAccessKey, ""),

		// 2. 🔒 安全传输：严格对应你原本最后的那个布尔值（false 代表走 http，true 代表走 https）
		Secure: false,
	})
	if err != nil {
		global.SugaredLogger.Fatalf("❌ [MinIO v7] 初始化客户端连接大翻车: %v", err)
	}
	fmt.Printf("%+v", minioClient)
	fmt.Println("Minio连接成功")
	global.MinioClient = minioClient
	utils.CreateMinoBuket("userheaders")
}
