package Init

import (
	"Go_Project/global"
	"Go_Project/utils"
	"fmt"
	"github.com/minio/minio-go"
	"log"
)

func InitMinio() {
	minioInfo := global.GLOB_CONFIG.Minio
	minioClient, err := minio.New(minioInfo.EndPoint, minioInfo.AccessKeyID, minioInfo.SecretAccessKey, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v", minioClient)
	fmt.Println("Minio连接成功")
	global.MinioClient = minioClient
	utils.CreateMinoBuket("userheaders")
}
