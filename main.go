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
	global.GVA_VP = core.Viper() // 启动viper(配置读取器)
	_ = global.GVA_VP
	logger.Init()
	Init.InitMinio()
	global.GVA_DB = Init.GormMySQL()
	Init.Redis()
	// 1. 声明全局的可取消上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// 2. 同步异步拉起 Kafka
	Init.InitKafka(ctx)

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("verifyMobileFormat", utils.VerifyMobileFormat)
		if err != nil {
			return
		}
	}
	srv := &http.Server{
		Addr:    global.GLOB_CONFIG.System.Port,
		Handler: Init.Routers(),
	}
	// 🌟 核心修改：把 Gin 的启动塞进协程里异步运行，防止它阻塞主线程
	go func() {
		log.Printf("📥 Web 服务正在启动，监听端口: %s\n", global.GLOB_CONFIG.System.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			global.SugaredLogger.Errorf("Web 服务异常关闭: %s\n", err)
		}
	}()
	// 🌟 此时主线程会非常安全地停在这里，静静等待 Ctrl+C 信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	fmt.Println("\n----------------------------------------------------------------")
	log.Println("👋 接收到关闭信号，正在触发全局优雅退出...")
	// 📥 第一步：先优雅关闭 Gin Web 服务（给还在路上的请求 5 秒宽限期）
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		global.SugaredLogger.Errorf("Gin Web 关闭失败: %v", err)
	} else {
		log.Println("✅ [Success] Gin Web 服务已安全关闭.")
	}
	// 🚀 第二步：触发 cancel()，此时 initKafka.go 里的 <-ctx.Done() 会立刻收到通知
	log.Println("🔄 正在向 Kafka 发送终止信号...")
	cancel()
	// 稍微等一下让 Kafka 把文件句柄写回硬盘，防止数据损坏
	wg.Wait()
	log.Println("🏁 所有人全部安全下线，主程序退出。")
}
