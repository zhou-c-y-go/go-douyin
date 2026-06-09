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
