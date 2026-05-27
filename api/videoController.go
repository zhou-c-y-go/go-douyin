package api

import (
	"Go_Project/common/model/response"
	"Go_Project/service"
	"github.com/gin-gonic/gin"
)

type VideoController struct{}

var videoService = new(service.VideoService)

func (api *VideoController) GetFeedList(c *gin.Context) {
	// 门卫处：通过 c.Request.Context() 把带有 TraceID 的标准上下文剥离出来送进去
	ctx := c.Request.Context()

	list, err := videoService.GetFeedListMiddle(ctx)
	if err != nil {
		response.Fail(c, response.ERROR, "拉取失败")
		return
	}

	response.Success(c, list)
}
