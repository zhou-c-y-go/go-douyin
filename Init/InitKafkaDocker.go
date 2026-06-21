package Init

import (
	"Go_Project/global"
	"log"
	"net"
	"strings"
	"time"
)

func getKafkaBootstrapServer() string {
	// 从 Viper 已经加载好的 global.GLOB_CONFIG 中读取 Kafka 参数
	bootstrapServers := strings.TrimSpace(global.GLOB_CONFIG.MQ.BootstrapServers)

	if bootstrapServers == "" {
		bootstrapServers = "127.0.0.1:9092"
	}

	// 如果以后配置多个 Kafka 地址，例如：
	// 127.0.0.1:9092,127.0.0.1:9093
	// 健康检查时先取第一个
	firstServer := strings.Split(bootstrapServers, ",")[0]
	return strings.TrimSpace(firstServer)
}

// Kafka 不再启动 Kafka 服务，只检查 Docker Kafka 是否就绪
func Kafka() {
	kafkaAddr := getKafkaBootstrapServer()

	log.Printf("⏳ 正在等待 Kafka 就绪，连接地址：%s", kafkaAddr)

	for {
		conn, err := net.DialTimeout("tcp", kafkaAddr, 5*time.Second)
		if err != nil {
			log.Printf("❌ [Failed] Kafka 暂未就绪：%s", err.Error())
			time.Sleep(2 * time.Second)
			continue
		}

		_ = conn.Close()
		log.Println("✨ [Success] Kafka 已就绪，Go 主服务可以启动！")
		break
	}
}
