package api

import (
	"Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type CommentController struct {
	commentService service.CommentService // 契约接口注入
}

func NewCommentController(cs service.CommentService) *CommentController {
	return &CommentController{
		commentService: cs,
	}
}

func (api *CommentController) GetVideoCommentTree(c *gin.Context) {
	videoIDStr := c.Query("video_id")
	videoID, err := strconv.ParseInt(videoIDStr, 10, 64)
	if err != nil || videoID <= 0 {
		response.Fail(c, response.ERROR, "无法捕获有效的视频定位单据")
		return
	}

	ctx := c.Request.Context()
	var currentUserID int64 = 0
	if claimInterface, exists := c.Get("claim"); exists {
		if claims, ok := claimInterface.(*request.CustomClaims); ok {
			currentUserID = claims.Id
		}
	}

	tree, err := api.commentService.GetVideoCommentTreeService(ctx, videoID, currentUserID)
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Comment] 聚拢树状评论大翻车: %v", err)
		response.Fail(c, response.ERROR, "评论区系统维护中")
		return
	}

	response.Success(c, tree)
}

func (api *CommentController) CreateComment(c *gin.Context) {
	claimInterface, _ := c.Get("claim")
	claims := claimInterface.(*request.CustomClaims)
	userID := claims.Id

	var req struct {
		VideoID   int64  `json:"video_id" binding:"required"`
		Content   string `json:"content" binding:"required"`
		ReplyToID int64  `json:"reply_to_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "评论包裹损坏，缺少关键单据"})
		return
	}

	ctx := c.Request.Context()
	err := api.commentService.PublishCommentService(ctx, req.VideoID, userID, req.Content, req.ReplyToID)
	if err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	response.Success(c, response.OK)
}
