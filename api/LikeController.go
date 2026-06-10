package api

import (
	req "Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
)

type LikeController struct {
	likeService service.LikeService // 组合点赞业务服务
}

// ToggleLike ── 🎯 对应前端：request.post('/like/toggle')
func (api *LikeController) ToggleLike(c *gin.Context) {
	// 1. 👮 严格搜身：从 JWT Token 中扒出当前点赞的真实操作人 ID
	// 绝对不信任前端传过来的 UserID，防范黑客伪造身份恶意刷赞！
	claimInterface, _ := c.Get("claim")
	claims := claimInterface.(*req.CustomClaims) // 精准对齐你的 request 包前缀
	userID := claims.Id

	// 2. 接收前端抛投的点赞包裹
	var req1 struct {
		TargetID   int64  `json:"target_id" binding:"required"`
		TargetType string `json:"target_type" binding:"required"` // "video" / "comment"
		Status     int8   `json:"status" binding:"oneof=0 1"`     // 🎯 核心引入：1为点赞，0为取消
	}
	if err := c.ShouldBindJSON(&req1); err != nil {
		global.LogCtx(c.Request.Context()).Errorf("❌ [Like-Bind-Error] JSON 参数解析硬性失败: %v", err)
		response.Fail(c, response.ERROR, "点赞包裹损坏，缺少关键单据")
		return
	}
	ctx := c.Request.Context()
	isLiked, err := api.likeService.ToggleLikeService(ctx, userID, req1.TargetID, req1.TargetType, int(req1.Status))
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Like] 翻转点赞状态大翻车: %v", err)
		response.Fail(c, response.ERROR, err.Error())
		return
	}

	// 4. 完美响应
	if isLiked {
		response.Success(c, gin.H{"is_liked": true})
	} else {
		response.Success(c, gin.H{"is_liked": false})
	}
}

// GetUserTotalLikeCountController ：统计用户点赞视频的数量
func (api *LikeController) GetUserTotalLikeCountController(c *gin.Context) {
	claimInterface, exists := c.Get("claim")
	if !exists {
		response.Fail(c, response.ERROR, "未登录")
		return
	}
	claims := claimInterface.(*req.CustomClaims) // 精准对齐你的 request 包前缀
	userID := claims.Id
	count, err := api.likeService.GetUserTotalLikeCount(c, userID)
	if err != nil {
		global.LogCtx(c).Errorf("用户[%d]点赞视频数量获取失败", userID)
		response.Fail(c, response.ERROR, err.Error())
	} else {
		response.Success(c, count)
	}
}

func (api *LikeController) CalibrateVideoCounts(c *gin.Context) {
	ctx := c.Request.Context()
	if err := api.likeService.CalibrateVideoCounts(ctx); err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	response.Success(c, gin.H{"msg": "视频计数校准完成"})
}
