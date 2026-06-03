package api

import (
	"Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
	"mime/multipart"
	"strconv"
)

type VideoController struct {
	videoService service.VideoService
}

// UploadVideo ── ✅ 大厂标准的视频+封面原子化聚合上传接口
func (api *VideoController) UploadVideo(c *gin.Context) {
	// 1. JWT 搜身安全提权，提取当前发帖人的真实 ID（严禁听信前端传参，死守安全红线）
	claimInterface, _ := c.Get("claim")
	claims := claimInterface.(*request.CustomClaims)
	authorID := claims.Id
	// 2. 扒出表单中文本和媒体流
	title := c.PostForm("title")
	tags := c.PostForm("tags")
	durationStr := c.PostForm("duration")
	duration, _ := strconv.ParseInt(durationStr, 10, 64)
	videoFile, err := c.FormFile("video")
	if err != nil {
		response.Fail(c, response.ERROR, "视频文件必传")
		return
	}
	coverFile, err := c.FormFile("cover")
	if err != nil {
		response.Fail(c, response.ERROR, "封面图片必传")
		return
	}
	// 3. 打开双流通道
	// 视频文件
	videoObj, err := videoFile.Open()
	if err != nil {
		response.Fail(c, response.ERROR, "解析视频流失败")
		return
	}
	defer func(videoObj multipart.File) {
		err := videoObj.Close()
		if err != nil {
			response.Fail(c, response.ERROR, "解析封面流失败")
			return
		}
	}(videoObj) // 💡 铁律：defer 清场，绝不留任何内存漏洞！

	// 视频封面文件
	coverObj, err := coverFile.Open()
	if err != nil {
		response.Fail(c, response.ERROR, "解析封面流失败")
		return
	}
	defer func(coverObj multipart.File) {
		err := coverObj.Close()
		if err != nil {
			response.Fail(c, response.ERROR, "解析封面流失败")
			return
		}
	}(coverObj)

	// 4. 获取带有日志追踪的 context
	ctx := c.Request.Context()
	global.LogCtx(ctx).Infof("📥 [Controller] 用户 [%d] 正在投递新视频: %s, 标签: %s", authorID, title, tags)
	if err := api.videoService.UploadVideoService(ctx, title, tags, videoFile, coverFile, videoObj, coverObj, authorID, duration); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}

	// 5. 返回代表大功告成的 1 暗号！
	response.Success(c, response.OK)
}

// GetFeedStream ── ✅ 高性能 Feed 视频流刷新接口
func (api *VideoController) GetFeedStream(c *gin.Context) {
	ctx := c.Request.Context()

	// 尝试解析当前的登录状态（非强求，没登录也能刷视频，只是点赞亮红心功能不激活）
	var currentUserID int64 = 0
	if claimInterface, exists := c.Get("claim"); exists {
		if claims, ok := claimInterface.(*request.CustomClaims); ok {
			currentUserID = claims.Id
		}
	}

	// 传唤组装厂进行多表内存并网
	videoVOs, err := api.videoService.GetFeedStreamService(ctx, currentUserID)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Controller] 刷新视频推荐流遭遇滑铁卢: %v", err)
		response.Fail(c, response.ERROR, "无法获取推荐视频流")
		return
	}
	response.Success(c, videoVOs)
}

// RepairDuration ── 承接前端自愈系统派发过来的老数据清洗指标
func (api *VideoController) RepairDuration(c *gin.Context) {
	// 声明轻量级接收结构体
	var req struct {
		ID       int64 `json:"id" binding:"required"`
		Duration int64 `json:"duration" binding:"required"`
	}

	// 如果前端传过来的 JSON 格式不对，直接优雅挂起
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ERROR, "参数有误")
		return
	}

	// 调用服务层，直接在底层对 MySQL 执行覆盖式刷数
	ctx := c.Request.Context()
	if err := api.videoService.RepairHistoricalDuration(ctx, req.ID, req.Duration); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}

	response.Success(c, response.OK)
}

// GetPresignedUploadURL ── 🎯 对应前端：request.post('/video/get-presigned-url')
func (api *VideoController) GetPresignedUploadURL(c *gin.Context) {
	var req struct {
		Bucket     string `json:"bucket" binding:"required"`
		ObjectName string `json:"object_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ERROR, "参数有误")
		return
	}

	ctx := c.Request.Context()
	// 签发 15 分钟时效的直传专属通行证
	urlStr, err := utils.GetUploadPresignedURL(ctx, req.Bucket, req.ObjectName, 15*time.Minute) //
	if err != nil {
		response.Fail(c, response.ERROR, "无法孵化直传通行证")
		return
	}

	response.Success(c, gin.H{"url": urlStr})
}

// InitMultipart ── 🎯 对应前端：request.post('/video/init-multipart')
func (api *VideoController) InitMultipart(c *gin.Context) {
	var req struct {
		ObjectName string `json:"object_name" binding:"required"`
		ChunkCount int    `json:"chunk_count" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ERROR, "初始化参数有误")
		return
	}

	ctx := c.Request.Context()
	// 1. 去 MinIO 申请一个本次上传唯一的 UploadID 大令牌
	uploadID, err := utils.InitMinioMultipartUpload(ctx, "videos", req.ObjectName)
	if err != nil {
		response.Fail(c, response.ERROR, "MinIO 桶分片链路建立失败")
		return
	}

	// 2. 顺着前端切出来的 ChunkCount 数量，通过循环给每一块分片批量生成独一无二的预签名直传链接！
	partURLs := make([]string, req.ChunkCount)
	for i := 1; i <= req.ChunkCount; i++ {
		// 分片序列号从 1 开始累加
		partURL, err := utils.GetPresignedUploadPartURL(ctx, "videos", req.ObjectName, uploadID, i, 30*time.Minute)
		if err != nil {
			response.Fail(c, response.ERROR, "批量孵化分片链接遭遇大出轨")
			return
		}
		partURLs[i-1] = partURL
	}

	// 3. 将大厂大礼包原封不动秒级甩回给前端
	response.Success(c, gin.H{
		"upload_id": uploadID,
		"urls":      partURLs,
	})
}

// CompleteMultipart ── 🎯 对应前端：request.post('/video/complete-multipart')
func (api *VideoController) CompleteMultipart(c *gin.Context) {
	// JWT 搜身安全提权，锁死操作人
	claimInterface, _ := c.Get("claim")              //
	claims := claimInterface.(*request.CustomClaims) //
	authorID := claims.Id                            //

	var req struct {
		UploadID   string `json:"upload_id" binding:"required"`
		ObjectName string `json:"object_name" binding:"required"`
		CoverName  string `json:"cover_name" binding:"required"`
		Title      string `json:"title" binding:"required"`
		Tags       string `json:"tags"`
		Duration   int64  `json:"duration"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ERROR, "合并入库校验单据缺失")
		return
	}

	ctx := c.Request.Context()
	global.LogCtx(ctx).Infof("🧱 [Controller] 收到分片收网指令！准备为用户 [%d] 合并大作: %s", authorID, req.Title)

	// 传唤 Service 服务层，开启“合并 ＋ 落盘 MySQL ＋ 缓存一致性擦除”豪华套餐
	if err := api.videoService.CompleteMultipartVideoService(ctx, req.UploadID, req.ObjectName, req.CoverName, req.Title, req.Tags, req.Duration, authorID); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}

	response.Success(c, response.OK) // 返回 1 成功暗号！
}
