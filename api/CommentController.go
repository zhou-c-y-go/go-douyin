package api

import (
	"Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

type CommentController struct {
	commentService service.CommentService
}

// GetVideoCommentTree ── 对应前端：request.get('/comment/tree?video_id=12')
func (api *CommentController) GetVideoCommentTree(c *gin.Context) {
	videoIDStr := c.Query("video_id")
	videoID, err := strconv.ParseInt(videoIDStr, 10, 64)
	if err != nil || videoID <= 0 {
		response.Fail(c, response.ERROR, "无法捕获有效的视频定位单据")
		return
	}

	ctx := c.Request.Context()
	tree, err := api.commentService.GetVideoCommentTreeService(ctx, videoID)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Comment] 聚拢树状评论大翻车: %v", err)
		response.Fail(c, response.ERROR, "评论区系统维护中")
		return
	}

	// 秒级吐回完美嵌套的用户头像与名字树！
	response.Success(c, tree)
}

// CreateComment ── 🎯 对应前端：request.post('/comment')
func (api *CommentController) CreateComment(c *gin.Context) {
	// 1. 👮 严格搜身：从 JWT Token 中扒出当前发评论的真实操作人 ID
	// 绝对不信任前端传过来的 UserID，防范黑客伪造身份发不良言论！
	claimInterface, _ := c.Get("claim")
	claims := claimInterface.(*request.CustomClaims)
	userID := claims.Id

	// 2. 接收前端抛投的评论包裹
	var req struct {
		VideoID   int64  `json:"video_id" binding:"required"`
		Content   string `json:"content" binding:"required"`
		ReplyToID int64  `json:"reply_to_id"` // 可选参数，传 0 代表回复视频本体
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ERROR, "评论包裹损坏，缺少关键单据")
		return
	}

	ctx := c.Request.Context()

	// 3. 传唤服务层起飞
	if err := api.commentService.PublishCommentService(ctx, req.VideoID, userID, req.Content, req.ReplyToID); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}

	// 4. 完美响应
	response.Success(c, "神评论已上墙！")
}
