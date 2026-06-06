package Init

import (
	"Go_Project/global"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

// startEmbeddedKafka 负责管理 Kafka 的生命周期
func startEmbeddedKafka(ctx context.Context) {
	kafkaInfo := global.GLOB_CONFIG.Kafka
	// 1. 定义你的 Kafka 根目录和相关路径（精准匹配你本地的路径）
	kafkaDir := kafkaInfo.KafkaDir
	batPath := kafkaInfo.BatPath
	configPath := kafkaInfo.ConfigPath
	// 2. 创建一个带 Context 的系统命令，这样方便后续跟随主进程一起退出
	cmd := exec.CommandContext(ctx, batPath, configPath)
	// 关键配置：必须把工作目录切换到 Kafka 根目录，否则它找不到相对路径下的数据盘
	cmd.Dir = kafkaDir
	// 3. 兜底保护：在环境变量里强行再注入一次内存配置，双重保险防止 wmic 报错
	cmd.Env = append(os.Environ(), "KAFKA_HEAP_OPTS=-Xmx1G -Xms1G")
	// 4. 重定向输出：把 Kafka 产生的日志，直接借用你当前 Go 程序的控制台打印出来
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("🚀 [Go-Orchestrator] 正在后台拉起 Kafka 4.0 服务...")

	// 5. 使用 Start() 异步启动，绝对不能用 Run()，因为 Run 会死锁阻塞住主线程
	if err := cmd.Start(); err != nil {
		log.Fatalf("❌ [Go-Orchestrator] Kafka 启动命令执行失败: %v", err)
	}
	// 6. 此时 Kafka 已在后台运行，在这里默默等待主程序结束的信号
	<-ctx.Done()
	log.Println("🛑 [Go-Orchestrator] 检测到主服务准备退出，正在强行释放 Kafka 子进程...")
	if cmd.Process != nil {
		// 结束 Kafka 进程
		_ = cmd.Process.Kill()
	}
}

func InitKafka(ctx context.Context) {
	// 🌟 核心：开启一个 Goroutine（协程）去异步同步启动 Kafka
	go startEmbeddedKafka(ctx)
	// 故意让主线程等一会（比如5秒），因为 Kafka 启动需要一点点时间初始化
	log.Println("⏳ 正在等待 Kafka 初始化就绪...")
	time.Sleep(5 * time.Second)

	log.Println("✨ [Success] Kafka 已同步拉起，你的 Go 主业务服务现在正式启动！")
	fmt.Println("----------------------------------------------------------------")
}
