package api

import (
	req "Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
)

type LikeController struct {
	likeService service.LikeService
}

func NewLikeController(ls service.LikeService) *LikeController {
	return &LikeController{likeService: ls}
}

func (api *LikeController) ToggleLike(c *gin.Context) {
	claimInterface, _ := c.Get("claim")
	claims := claimInterface.(*req.CustomClaims)
	userID := claims.Id

	var req1 struct {
		TargetID   int64  `json:"target_id" binding:"required"`
		TargetType string `json:"target_type" binding:"required"`
		Status     int8   `json:"status" binding:"oneof=0 1"`
		AuthorID   int64  `json:"author_id" binding:"oneof=0 1"`
	}
	if err := c.ShouldBindJSON(&req1); err != nil {
		response.Fail(c, response.ERROR, "点赞包裹损坏，缺少关键单据")
		return
	}

	ctx := c.Request.Context()
	isLiked, err := api.likeService.ToggleLikeService(ctx, userID, req1.TargetID, req1.AuthorID, req1.TargetType, int(req1.Status))
	if err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}

	response.Success(c, gin.H{"is_liked": isLiked})
}

func (api *LikeController) CalibrateVideoCounts(c *gin.Context) {
	ctx := c.Request.Context()
	if err := api.likeService.CalibrateVideoCounts(ctx); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	response.Success(c, gin.H{"msg": "视频计数校准完成"})
}
