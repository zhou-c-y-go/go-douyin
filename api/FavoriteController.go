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
	return &FavoriteController{favorService: fs} //[cite: 12]
}

func (api *FavoriteController) ToggleFavorite(c *gin.Context) {
	claimInterface, _ := c.Get("claim")          //[cite: 12]
	claims := claimInterface.(*req.CustomClaims) //[cite: 12]
	userID := claims.Id                          //[cite: 12]

	var req1 struct {
		TargetID   int64  `json:"target_id" binding:"required"`   //[cite: 12]
		TargetType string `json:"target_type" binding:"required"` //[cite: 12]
		Status     int8   `json:"status" binding:"oneof=0 1"`     //[cite: 12]
	}
	if err := c.ShouldBindJSON(&req1); err != nil { //[cite: 12]
		response.Fail(c, response.ERROR, "收藏包裹损坏，缺少关键单据") //[cite: 12]
		return
	}

	ctx := c.Request.Context()                                                                                             //[cite: 12]
	isFavorited, err := api.favorService.ToggleFavorService(ctx, userID, req1.TargetID, req1.TargetType, int(req1.Status)) //[cite: 12]
	if err != nil {
		response.Fail(c, response.ERROR, err.Error()) //[cite: 12]
		return
	}

	response.Success(c, gin.H{"is_favorite": isFavorited}) //[cite: 12]
}
