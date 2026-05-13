package utils

import (
	"Go_Project/global"
	"fmt"
	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/policy"
	"io"
	"net/url"
	"time"
)

func CreateMinoBuket(bucketName string) {
	location := "us-east-1"
	err := global.MinioClient.MakeBucket(bucketName, location)
	fmt.Println(err)
	if err != nil {
		exists, err := global.MinioClient.BucketExists(bucketName)
		global.SugaredLogger.Error(err)
		if err == nil && exists {
			fmt.Printf("we already own %s\n", bucketName)
		} else {
			global.SugaredLogger.Errorf("发生错误：%v, %v", err, exists)
			return
		}
	}
	err = global.MinioClient.SetBucketPolicy(bucketName, policy.BucketPolicyReadWrite)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Successfully created %s\n", bucketName)
}

func UpLoadFile(buckName string, objectName string, reader io.Reader, objectSize int64) (ok bool) {
	n, err := global.MinioClient.PutObject(buckName, objectName, reader, objectSize, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		fmt.Println(err)
		return false
	}
	fmt.Println("Successfully uploaded bytes: ", n)
	return true
}

func GetFileURL(bucketName string, fileName string, expires time.Duration) string {
	reqParams := make(url.Values)
	presignedURL, err := global.MinioClient.PresignedGetObject(bucketName, fileName, expires, reqParams)
	if err != nil {
		global.SugaredLogger.Errorf("%s", err)
		return ""
	}
	return fmt.Sprintf("%s", presignedURL)
}
