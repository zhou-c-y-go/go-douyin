package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"strings"
)

type user struct {
	Tel string `json:"tel" binding:"required,verifyMobileFormat"`
}

func main() {
	r := gin.Default()
	r.StaticFile("/v1/user/", "../static/headImags/admin1.jpg")
	r.GET("v1/x/test", func(c *gin.Context) {
		c.String(200, "hello")
		a, _ := strings.CutPrefix(c.Request.URL.Path, "")
		fmt.Println(c.Request.URL)
		fmt.Println(a)
	})
	//file, _ := c.FormFile("head-img")
	//dst := "./" + file.Filename
	//fmt.Println(s)
	r.Run(":9090")
}
