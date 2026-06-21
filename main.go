package main

import (
	"Go_Project/Init"
	"Go_Project/core"
	"Go_Project/global"
	"Go_Project/logger"
	"Go_Project/utils"
	"context"
	"fmt"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 启动 Viper，读取 config.yaml
	_ = core.Viper()
	// 初始化日志
	logger.Init()
	// 初始化依赖服务客户端
	Init.InitMinio()
	// 初始化数据库配置
	global.GVA_DB = Init.InitDatabaseFactory()
	Init.RegisterAutoMigrateTable(global.GVA_DB)
	// 初始化redis缓存
	Init.Redis()
	// 等待 Docker Kafka 就绪
	// 现在 Kafka 由 docker-compose 管理，Go 这里只做连接检查
	//Init.Kafka()
	// 注册自定义校验器
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("verifyMobileFormat", utils.VerifyMobileFormat)
		if err != nil {
			log.Printf("注册手机号校验器失败: %v", err)
			return
		}
	}
	// 初始化 HTTP Server
	srv := &http.Server{
		Addr:    global.GLOB_CONFIG.System.Port,
		Handler: Init.Routers(),
	}

	// 启动 Web 服务
	go func() {
		log.Printf("📥 Web 服务正在启动，监听端口: %s\n", global.GLOB_CONFIG.System.Port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			global.SugaredLogger.Errorf("Web 服务异常关闭: %s\n", err)
		}
	}()

	// 等待 Ctrl+C 或系统终止信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	fmt.Println("\n----------------------------------------------------------------")
	log.Println("👋 接收到关闭信号，正在优雅关闭服务...")

	// 优雅关闭 Web 服务
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		global.SugaredLogger.Errorf("Gin Web 关闭失败: %v", err)
	} else {
		log.Println("✅ [Success] Gin Web 服务已安全关闭.")
	}

	log.Println("🏁 主程序退出。")
}
