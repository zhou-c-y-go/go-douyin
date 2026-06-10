package api

import (
	req "Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
)

type FavoriteController struct {
	favorService service.FavorService
}

func (s *FavoriteController) ToggleFavorite(c *gin.Context) {
	claimInterface, _ := c.Get("claim")
	claims := claimInterface.(*req.CustomClaims) // 精准对齐你的 request 包前缀
	userID := claims.Id

	// 2. 接收前端抛投的收藏包裹
	var req1 struct {
		TargetID   int64  `json:"target_id" binding:"required"`
		TargetType string `json:"target_type" binding:"required"` // "video" / "comment"
		Status     int8   `json:"status" binding:"oneof=0 1"`     // 🎯 核心引入：1为收藏，0为取消
	}
	if err := c.ShouldBindJSON(&req1); err != nil {
		global.LogCtx(c.Request.Context()).Errorf("❌ [Favorite-Bind-Error] JSON 参数解析硬性失败: %v", err)
		response.Fail(c, response.ERROR, "收藏包裹损坏，缺少关键单据")
		return
	}
	ctx := c.Request.Context()
	isFavorited, err := s.favorService.ToggleFavorService(ctx, userID, req1.TargetID, req1.TargetType, int(req1.Status))
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Favorite] 翻转收藏状态大翻车: %v", err)
		response.Fail(c, response.ERROR, err.Error())
		return
	}

	// 4. 完美响应
	if isFavorited {
		response.Success(c, gin.H{"is_favorite": true})
	} else {
		response.Success(c, gin.H{"is_favorite": false})
	}
}

func (api *FavoriteController) GetUserTotalFavoriteCountController(c *gin.Context) {
	claimInterface, exists := c.Get("claim")
	if !exists {
		response.Fail(c, response.ERROR, "未登录")
		return
	}
	claims := claimInterface.(*req.CustomClaims) // 精准对齐你的 request 包前缀
	userID := claims.Id
	count, err := api.favorService.GetFavoriteTotalFavoriteCount(c, userID)
	if err != nil {
		global.LogCtx(c).Errorf("用户[%d]点赞视频数量获取失败", userID)
		response.Fail(c, response.ERROR, err.Error())
	} else {
		response.Success(c, count)
	}
}
