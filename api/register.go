package api

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/model/request"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"Go_Project/service"
	"Go_Project/utils"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
	"net/http"
)

var usrService service.UserService

type BaseService struct {
}

func (b *BaseService) Register(c *gin.Context) {
	var r request.Register
	if err := utils.InitTrans("zh"); err != nil {
		fmt.Println("翻译器初始化失败:", err)
	}
	if err := c.ShouldBind(&r); err != nil {
		errs, ok := err.(validator.ValidationErrors)
		if !ok {
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":          response.ERROR,
			"error-message": errs.Translate(utils.Trans),
			"data":          nil,
		})
		return
	}
	user := &pojo.User{
		Username:  r.Username,
		Password:  r.Password,
		Email:     r.Email,
		Telephone: r.Telephone,
		Status:    r.Status,
	}
	userService := service.UserService{}
	userReturn, err := userService.Register(*user)
	if err != nil {
		global.SugaredLogger.Error("注册失败!", zap.Error(err))
		response.Fail(c, response.ERROR, "注册失败!")
		return
	}
	response.Success(c, userReturn)
}

func (b *BaseService) ResetPassword(c *gin.Context) {
	var user pojo.User
	err := c.ShouldBindJSON(&user)
	if err != nil {
		response.Fail(c, response.ERROR, err.Error())
		return
	}
	err = usrService.ResetPassword(uint(user.ID))
	if err != nil {
		global.SugaredLogger.Error("重置失败!", zap.Error(err))
		response.Fail(c, response.ERROR, "重置失败"+err.Error())
		return
	}
	response.Success(c, "重置成功")
}
