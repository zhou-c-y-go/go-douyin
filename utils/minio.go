package utils

import (
	"Go_Project/global"
	"context"
	"fmt"
	"github.com/minio/minio-go/v7" // 👑 唯一种姓：只认纯正的 v7 绝对路径，彻底轰碎分裂症！
	"io"
	"net/url"
	"time"
)

// CreateMinoBuket ── 🎬 v7版本：创建桶并赋予标准的公网读写/下载策略
func CreateMinoBuket(bucketName string) {
	// 1. 创建一个后台通用的 context
	ctx := context.Background()

	// 🎯 v7 核心修正：第三个参数使用标准的 minio.MakeBucketOptions 结构体
	err := global.MinioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		exists, errBucketEx := global.MinioClient.BucketExists(ctx, bucketName)
		if errBucketEx == nil && exists {
			fmt.Printf("💡 [MinIO v7] 我们已经拥有此存储桶: %s，跳过创建步骤。\n", bucketName)
		} else {
			global.SugaredLogger.Errorf("❌ [MinIO v7] 创建存储桶发生毁灭性异常：%v", err)
			return
		}
	}

	// 🎯 v7 黄金升级：废弃原先垃圾的 policy 依赖包，直接使用大厂通用的标准可读写 JSON 字符串！
	// 这样可以确保视频和封面能够被前端免鉴权、零阻碍地高速拉取
	policyJSON := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetBucketLocation", "s3:ListBucket", "s3:ListBucketMultipartUploads"],
				"Resource": ["arn:aws:s3:::%s"]
			},
			{
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:AbortMultipartUpload", "s3:ListMultipartUploadParts"],
				"Resource": ["arn:aws:s3:::%s/*"]
			}
		]
	}`, bucketName, bucketName)

	// 注入全新的策略 JSON 文本
	err = global.MinioClient.SetBucketPolicy(ctx, bucketName, policyJSON)
	if err != nil {
		global.SugaredLogger.Errorf("❌ [MinIO v7] 注入桶公网策略失败：%v", err)
		return
	}
	fmt.Printf("✅ [MinIO v7] 成功创建并配置公网读写桶：%s\n", bucketName)
}

// UpLoadFile ── 🚀 v7版本：带 ctx 上下文和标准 Options 的全功能多媒体流投递器
func UpLoadFile(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64) bool {
	// 🎯 对齐 v7 签名：ctx ＋ 桶名 ＋ 对象名 ＋ 结构体配置项
	_, err := global.MinioClient.PutObject(ctx, bucketName, objectName, reader, objectSize, minio.PutObjectOptions{
		ContentType: "application/octet-stream", // 大厂规范：统一声明为通用的二进制字节流
	})

	if err != nil {
		fmt.Println("❌ [MinIO v7] 文件推入存储桶失败:", err)
		return false
	}
	return true
}

// GetFileURL ── 🛹 v7版本：获取带有时效防盗链的专属下载链接
func GetFileURL(ctx context.Context, bucketName string, fileName string, expires time.Duration) string {
	reqParams := make(url.Values)

	// 🎯 对齐 v7 签名：PresignedGetObject 全面并网上下文控制
	presignedURL, err := global.MinioClient.PresignedGetObject(ctx, bucketName, fileName, expires, reqParams)
	if err != nil {
		global.SugaredLogger.Errorf("❌ [MinIO v7] 生成时效防盗链链接失败: %v", err)
		return ""
	}
	return presignedURL.String() // 💡 完美返回标准的纯文本 URL 链路
}

// EnsureBucketExists ── 👑 v7版本：创作者后台核心自愈防御安检雷达
func EnsureBucketExists(ctx context.Context, bucketName string) error {
	minioClient := global.MinioClient

	// 1. 探针雷达全速扫描
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [MinIO v7] 探测桶 [%s] 是否存在时发生异常: %v", bucketName, err)
		return err
	}

	// 2. 顺水推舟：发现漏网之鱼，原地用 v7 配置项进行紧急孵化
	if !exists {
		global.LogCtx(ctx).Warnf("⚠️ [MinIO v7] 监测到核心存储桶 [%s] 居然不存在！正在启动全自动并网逻辑...", bucketName)

		// 🎯 核心升级：改用 v7 的 Options 模式建桶
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
		if err != nil {
			global.LogCtx(ctx).Errorf("❌ [MinIO v7] 紧急创建桶 [%s] 彻底失败: %v", bucketName, err)
			return err
		}
		global.LogCtx(ctx).Infof("✅ [MinIO v7] 核心存储桶 [%s] 已全自动初始化成功，安全通道正式放行！", bucketName)
	}

	return nil
}
