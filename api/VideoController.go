package api

import (
	"Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/service"
	"Go_Project/utils"
	"github.com/gin-gonic/gin"
	"mime/multipart"
	"strconv"
	"time"
)

type VideoController struct {
	videoService service.VideoService // 升级为标准组件依赖注入接口
}

func NewVideoController(vs service.VideoService) *VideoController {
	return &VideoController{videoService: vs}
}

func (api *VideoController) UploadVideo(c *gin.Context) {
	claimInterface, _ := c.Get("claim")
	claims := claimInterface.(*request.CustomClaims)
	authorID := claims.Id

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

	videoObj, err := videoFile.Open()
	if err != nil {
		response.Fail(c, response.ERROR, "解析视频流失败")
		return
	}
	defer func(videoObj multipart.File) { _ = videoObj.Close() }(videoObj)

	coverObj, err := coverFile.Open()
	if err != nil {
		response.Fail(c, response.ERROR, "解析封面流失败")
		return
	}
	defer func(coverObj multipart.File) { _ = coverObj.Close() }(coverObj)

	ctx := c.Request.Context()
	if err := api.videoService.UploadVideoService(ctx, title, tags, videoFile, coverFile, videoObj, coverObj, authorID, duration); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	response.Success(c, response.OK)
}

func (api *VideoController) GetFeedStream(c *gin.Context) {
	ctx := c.Request.Context()
	var currentUserID int64 = 0
	if claimInterface, exists := c.Get("claim"); exists {
		if claims, ok := claimInterface.(*request.CustomClaims); ok {
			currentUserID = claims.Id
		}
	}

	videoVOs, err := api.videoService.GetFeedStreamService(ctx, currentUserID)
	if err != nil {
		response.Fail(c, response.ERROR, "无法获取推荐视频流")
		return
	}
	response.Success(c, videoVOs)
}

func (api *VideoController) RepairDuration(c *gin.Context) {
	var req struct {
		ID       int64 `json:"id" binding:"required"`
		Duration int64 `json:"duration" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ERROR, "参数有误")
		return
	}

	ctx := c.Request.Context()
	if err := api.videoService.RepairHistoricalDuration(ctx, req.ID, req.Duration); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	response.Success(c, response.OK)
}

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
	urlStr, err := utils.GetUploadPresignedURL(ctx, req.Bucket, req.ObjectName, 15*time.Minute)
	if err != nil {
		response.Fail(c, response.ERROR, "无法孵化直传通行证")
		return
	}
	response.Success(c, gin.H{"url": urlStr})
}

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
	uploadID, err := utils.InitMinioMultipartUpload(ctx, "videos", req.ObjectName)
	if err != nil {
		response.Fail(c, response.ERROR, "MinIO 桶分片链路建立失败")
		return
	}

	partURLs := make([]string, req.ChunkCount)
	for i := 1; i <= req.ChunkCount; i++ {
		partURL, err := utils.GetPresignedUploadPartURL(ctx, "videos", req.ObjectName, uploadID, i, 30*time.Minute)
		if err != nil {
			response.Fail(c, response.ERROR, "批量孵化分片链接失败")
			return
		}
		partURLs[i-1] = partURL
	}
	response.Success(c, gin.H{"upload_id": uploadID, "urls": partURLs})
}

func (api *VideoController) CompleteMultipart(c *gin.Context) {
	claimInterface, _ := c.Get("claim")
	claims := claimInterface.(*request.CustomClaims)
	authorID := claims.Id

	var req struct {
		UploadID   string `json:"upload_id" binding:"required"`
		ObjectName string `json:"object_name" binding:"required"`
		CoverName  string `json:"cover_name" binding:"required"`
		Title      string `json:"title" binding:"required"`
		Tags       string `json:"tags"`
		Duration   int64  `json:"duration"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ERROR, "合并入库单据缺失")
		return
	}

	ctx := c.Request.Context()
	if err := api.videoService.CompleteMultipartVideoService(ctx, req.UploadID, req.ObjectName, req.CoverName, req.Title, req.Tags, req.Duration, authorID); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	response.Success(c, response.OK)
}

func (api *VideoController) GetUserVideoList(c *gin.Context) {
	userIDStr := c.Query("user_id")
	targetUserID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || targetUserID <= 0 {
		response.Fail(c, response.ERROR, "非法的查阅用户对象")
		return
	}
	ctx := c.Request.Context()
	videoVOs, err := api.videoService.GetUserVideoListService(ctx, targetUserID)
	if err != nil {
		global.LogCtx(ctx).Errorw("无法获取该创作者的作品集", "Error", err)
		response.Fail(c, response.ERROR, "无法获取该创作者的作品集")
		return
	}
	response.Success(c, videoVOs)
}

func (api *VideoController) GetVideoDetail(c *gin.Context) {
	idStr := c.Query("id")
	videoID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || videoID <= 0 {
		response.Fail(c, response.ERROR, "视频通道定位损毁")
		return
	}

	var currentUserID int64 = 0
	if claimInterface, exists := c.Get("claim"); exists {
		if claims, ok := claimInterface.(*request.CustomClaims); ok {
			currentUserID = claims.Id
		}
	}
	ctx := c.Request.Context()
	videoVO, err := api.videoService.GetVideoDetailService(ctx, videoID, currentUserID)
	if err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	response.Success(c, videoVO)
}

func (api *VideoController) GetUserLikedVideoList(c *gin.Context) {
	userIDStr := c.Query("user_id")
	targetUserID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || targetUserID <= 0 {
		response.Fail(c, response.ERROR, "非法的查阅用户对象")
		return
	}

	// 🤫 探针微操：安全扒出当前正在看网页的登录用户 ID
	var currentUserID int64 = 0
	if claimInterface, exists := c.Get("claim"); exists {
		if claims, ok := claimInterface.(*request.CustomClaims); ok {
			currentUserID = claims.Id
		}
	}

	ctx := c.Request.Context()
	// 💡 并网投递：把被看的人（targetUserID）和正在看的人（currentUserID）一起塞进服务层
	videoVOs, err := api.videoService.GetUserLikeVideoListService(ctx, targetUserID, currentUserID)
	if err != nil {
		response.Fail(c, response.ERROR, "无法获取该创作者的喜欢集")
		return
	}

	response.Success(c, videoVOs)
}

func (api *VideoController) GetUserFavoriteVideoList(c *gin.Context) {
	userIDStr := c.Query("user_id")
	targetUserID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || targetUserID <= 0 {
		response.Fail(c, response.ERROR, "非法的查阅用户对象")
		return
	}

	// 🤫 探针微操：从 JWT 中扒出当前正在看网页的登录用户 ID
	var currentUserID int64 = 0
	if claimInterface, exists := c.Get("claim"); exists {
		if claims, ok := claimInterface.(*request.CustomClaims); ok {
			currentUserID = claims.Id
		}
	}

	ctx := c.Request.Context()
	// 💡 并网投递：把被看的人（targetUserID）和正在看的人（currentUserID）一起塞进去
	videoVOs, err := api.videoService.GetUserFavoriteVideoListService(ctx, targetUserID, currentUserID)
	if err != nil {
		response.Fail(c, response.ERROR, "无法获取该创作者的收藏集")
		return
	}

	response.Success(c, videoVOs)
}
