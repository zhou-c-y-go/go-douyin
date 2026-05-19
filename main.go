package main

import (
	"Go_Project/Init"
	"Go_Project/core"
	"Go_Project/global"
	"Go_Project/logger"
	"Go_Project/utils"
	"fmt"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func main() {
	global.GVA_VP = core.Viper() // 启动viper(配置读取器)
	logger.Init()
	fmt.Println("日志初试化完成")
	//Init.InitMinio()
	global.GVA_DB = Init.GormMySQL()
	fmt.Println("数据库初试化完成")
	Init.Redis()
	fmt.Println("Redis初试化完成")
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("verifyMobileFormat", utils.VerifyMobileFormat)
		if err != nil {
			return
		}
	}
	Init.Routers().Run(global.GLOB_CONFIG.System.Port)
}
