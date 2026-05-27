package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/common/model/response"
	"Go_Project/global"
	"fmt"
	"github.com/gin-gonic/gin"
	"strconv"
)

// QueryUserService 通过id查询用户
func (s *UserService) QueryUserService(c *gin.Context) {
	var user pojo.User
	if err := c.ShouldBindJSON(&user); err != nil {
		fmt.Println(err)
		response.Fail(c, response.ERROR, "发生错误")
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	user = repository.QueryByID(id)
	if user == (pojo.User{}) {
		response.Fail(c, response.ERROR, "未找到该用户")
	} else {
		response.Success(c, user)
	}
}

// QueryAll 查询所有的用户
func (s *UserService) QueryAll(c *gin.Context) {
	var users []pojo.User
	users = repository.QueryList()
	if users == nil {
		response.Fail(c, response.ERROR, "查询失败")
	} else {
		response.Success(c, users)
	}
}

// Delete 根据id删除用户
func (s *UserService) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	rows := repository.Delete(id)
	if rows > 0 {
		response.Success(c, "删除成功")
		global.SugaredLogger.Infof("%#v 正在试图删除用户信息", c.ClientIP())
	} else {
		response.Fail(c, response.ERROR, "无法找到该信息")
	}

}
