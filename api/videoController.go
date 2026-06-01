package api

import (
	"Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
	"mime/multipart"
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
	global.LogCtx(ctx).Infof("📥 [Controller] 用户 [%d] 正在投递新视频: %s", authorID, title)
	if err := api.videoService.UploadVideoService(ctx, title, videoFile, coverFile, videoObj, coverObj, authorID); err != nil {
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

	// 完美对齐大厂契约：返回 code:1 和组装好的豪华 VO 数组！
	response.Success(c, videoVOs)
}
