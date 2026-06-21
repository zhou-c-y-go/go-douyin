package api

import (
	req "Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
)

type FavoriteController struct {
	favorService service.FavorService
}

func NewFavoriteController(fs service.FavorService) *FavoriteController {
	return &FavoriteController{favorService: fs}
}

func (api *FavoriteController) ToggleFavorite(c *gin.Context) {
	claimInterface, _ := c.Get("claim")
	claims := claimInterface.(*req.CustomClaims)
	userID := claims.Id

	var req1 struct {
		TargetID   int64  `json:"target_id" binding:"required"`
		TargetType string `json:"target_type" binding:"required"`
		Status     int8   `json:"status" binding:"oneof=0 1"`
	}
	if err := c.ShouldBindJSON(&req1); err != nil {
		response.Fail(c, response.ERROR, "收藏包裹损坏，缺少关键单据")
		return
	}

	ctx := c.Request.Context()
	isFavorited, err := api.favorService.ToggleFavorService(ctx, userID, req1.TargetID, req1.TargetType, int(req1.Status))
	if err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}

	response.Success(c, gin.H{"is_favorite": isFavorited})
}

func (api *FavoriteController) GetUserTotalFavoriteCountController(c *gin.Context) {
	claimInterface, exists := c.Get("claim")
	if !exists {
		response.Fail(c, response.ERROR, "未登录")
		return
	}
	claims := claimInterface.(*req.CustomClaims)
	userID := claims.Id

	count, err := api.favorService.GetFavoriteTotalFavoriteCount(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	response.Success(c, count)
}
