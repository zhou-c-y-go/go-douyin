package response

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type BusinessCode = int32

const (
	OK    BusinessCode = 1
	ERROR BusinessCode = 0
)

type Response struct {
	Code    int32       `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"error-message"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		OK,
		data,
		"ok",
	})
}

func Fail(c *gin.Context, _ BusinessCode, msg string) {
	c.JSON(http.StatusBadRequest, Response{
		ERROR,
		nil,
		msg,
	})
}

// UserProfileResponse 定义返回给前端的专属数据结构，做好字段名映射
type UserProfileResponse struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	Avatar          string `json:"avatar"` // 对应后端的 HeadImg
	BackgroundImage string `json:"background_image"`
	Signature       string `json:"signature"`
	TotalLiked      int64  `json:"total_liked"` // 对应前端的 total_liked (后端 TotalFavorited)
	WorkCount       int64  `json:"work_count"`
	FavoriteCount   int64  `json:"favorite_count"`
}
