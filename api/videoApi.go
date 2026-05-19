package api

import (
	"github.com/gin-gonic/gin"
)

type VideoService struct{}

var baseService = new(BaseService)

func (api *VideoService) GetFeedList(c *gin.Context) {
	// 门卫处：通过 c.Request.Context() 把带有 TraceID 的标准上下文剥离出来送进去
	ctx := c.Request.Context()

	list, err := baseService.GetFeedListMiddle(ctx)
	if err != nil {
		c.JSON(500, gin.H{"msg": "拉取失败"})
		return
	}

	c.JSON(200, list)
}
