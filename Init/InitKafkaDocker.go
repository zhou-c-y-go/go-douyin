package Init

import (
	"context"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// getKafkaBootstrapServer 获取 Kafka 连接地址
// 本机运行 Go 项目时：127.0.0.1:9092
// Go 项目也在 Docker 容器内运行时：kafka:9092
func getKafkaBootstrapServer() string {
	bootstrapServers := strings.TrimSpace(os.Getenv("KAFKA_BOOTSTRAP_SERVERS"))

	if bootstrapServers == "" {
		bootstrapServers = strings.TrimSpace(os.Getenv("KAFKA_ADDR"))
	}

	if bootstrapServers == "" {
		bootstrapServers = "127.0.0.1:9092"
	}

	// 如果配置了多个地址，只取第一个做健康检查
	firstServer := strings.Split(bootstrapServers, ",")[0]
	return strings.TrimSpace(firstServer)
}

// KafkaDocker 不再负责启动 Kafka，只负责等待 Kafka 就绪
func KafkaDocker(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		kafkaAddr := getKafkaBootstrapServer()

		log.Printf("⏳ 正在等待 Kafka 就绪，连接地址：%s", kafkaAddr)

		for {
			select {
			case <-ctx.Done():
				log.Println("🛑 Kafka 等待任务已退出")
				return
			default:
				conn, err := net.DialTimeout("tcp", kafkaAddr, 5*time.Second)
				if err != nil {
					log.Printf("❌ [Failed] Kafka 暂未就绪：%s", err.Error())
					time.Sleep(2 * time.Second)
					continue
				}

				_ = conn.Close()
				log.Println("✨ [Success] Kafka 已就绪，你的 Go 主业务服务现在可以启动！")
				return
			}
		}
	}()
}
