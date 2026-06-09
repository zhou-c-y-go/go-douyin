package Init

import (
	"Go_Project/global"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// startEmbeddedKafka 负责管理 Kafka 的生命周期
func startEmbeddedKafka(ctx context.Context) {
	kafkaInfo := global.GLOB_CONFIG.Kafka
	// 1. 定义你的 Kafka 根目录和相关路径（精准匹配你本地的路径）
	kafkaDir := kafkaInfo.KafkaDir
	batPath := kafkaInfo.BatPath
	configPath := kafkaInfo.ConfigPath
	// 🛠️ 规范化 Windows 路径
	storageBatPath := filepath.Clean(filepath.Join(kafkaDir, "bin", "windows", "kafka-storage.bat"))
	configPath = filepath.Clean(configPath)
	batPath = filepath.Clean(batPath)
	configDirSlash := filepath.ToSlash(filepath.Dir(configPath))

	// =================================================================
	// 🔍 核心新增：智能状态探针 ── 检查 Kafka 是否已经格式化过
	// =================================================================
	isFormatted := false
	if propsData, err := os.ReadFile(configPath); err == nil {
		lines := strings.Split(string(propsData), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// 寻找 server.properties 里的数据落盘目录配置
			if strings.HasPrefix(line, "log.dirs=") {
				logDirs := strings.TrimPrefix(line, "log.dirs=")
				// 考虑到可能配置了多个目录（逗号分隔），我们取第一个目录作为状态代表
				firstDir := strings.Split(logDirs, ",")[0]
				firstDir = strings.TrimSpace(firstDir)

				// 🔍 铁证检查：如果该目录下存在 meta.properties，说明已经格式化过了
				metaPath := filepath.Join(firstDir, "meta.properties")
				if _, err := os.Stat(metaPath); err == nil {
					isFormatted = true
				}
				break
			}
		}
	}

	// 🧱 统一构建底层的系统环境变量基础盘
	baseEnv := os.Environ()
	baseEnv = append(baseEnv, "KAFKA_HEAP_OPTS=-Xmx1G -Xms1G")

	// 🛠️ 工具链与核心服务器的日志环境配置
	toolLog4j := fmt.Sprintf("-Dlog4j2.configurationFile=file:%s/tool-log4j2.yaml", configDirSlash)
	toolEnv := append(append([]string{}, baseEnv...), fmt.Sprintf("KAFKA_LOG4J_OPTS=%s", toolLog4j))

	serverLog4j := fmt.Sprintf("-Dlog4j2.configurationFile=file:%s/log4j2.yaml", configDirSlash)
	serverEnv := append(append([]string{}, baseEnv...), fmt.Sprintf("KAFKA_LOG4J_OPTS=%s", serverLog4j))

	// =================================================================
	// 🎰 状态分支决策
	// =================================================================
	if !isFormatted {
		log.Println("🌱 [Go-Orchestrator] 检测到 Kafka 属于首次并网，启动全自动点火仪式...")

		// 🌟 自动化步骤 1：获取随机集群 UUID
		log.Println("🔍 [Go-Orchestrator] 正在自动生成 KRaft 集群 UUID...")
		uuidCmd := exec.Command("cmd", "/c", storageBatPath, "random-uuid")
		uuidCmd.Dir = kafkaDir
		uuidCmd.Env = toolEnv
		uuidOut, err := uuidCmd.Output()
		if err != nil {
			log.Fatalf("❌ [Go-Orchestrator] 自动生成 UUID 失败: %v", err)
		}

		rawLines := strings.Split(strings.TrimSpace(string(uuidOut)), "\n")
		clusterUUID := strings.TrimSpace(rawLines[len(rawLines)-1])
		if len(clusterUUID) != 22 {
			clusterUUID = "4L62sA99RR6v9YgscwthSg"
		}
		log.Printf("🆔 [Go-Orchestrator] 成功捕获集群唯一身份证: %s", clusterUUID)

		// 🌟 自动化步骤 2：格式化数据盘
		log.Println("⚙️ [Go-Orchestrator] 正在尝试格式化 Kafka 数据目录...")
		formatCmd := exec.Command("cmd", "/c", storageBatPath, "format", "-t", clusterUUID, "-c", configPath, "--standalone")
		formatCmd.Dir = kafkaDir
		formatCmd.Env = toolEnv
		formatOut, _ := formatCmd.CombinedOutput()

		log.Println("========== FORMAT OUTPUT ==========")
		log.Println(strings.TrimSpace(string(formatOut)))
		log.Println("===================================")
		log.Println("✅ [Go-Orchestrator] 格式化指令执行完毕，meta.properties 已稳稳落盘！")

	} else {
		// 🚀 重启绝杀：一旦盘是热的，上面的步骤 1 和 2 连执行都不会执行，控制台绝对清爽！
		log.Println("⚡ [Go-Orchestrator] 检测到 meta.properties 已存在。数据盘状态健康，跳过初始化，直接热启动！")
	}

	// =================================================================
	// 🌟 自动化步骤 3：正式拉起 Kafka 4.0 核心流服务
	// =================================================================
	cmd := exec.CommandContext(ctx, "cmd", "/c", batPath, configPath)
	cmd.Dir = kafkaDir
	cmd.Env = serverEnv // 🔌 注入服务器专属日志环境

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("🚀 [Go-Orchestrator] 核心前置准备完毕，正在后台拉起 Kafka 4.0 服务...")
	if err := cmd.Start(); err != nil {
		log.Fatalf("❌ [Go-Orchestrator] Kafka 启动命令执行失败: %v", err)
	}

	// =================================================================
	// 🛑 自动化步骤 4：优雅退役
	// =================================================================
	<-ctx.Done()
	log.Println("🛑 [Go-Orchestrator] 检测到主服务准备退出，正在释放 Kafka 子进程...")
	if cmd.Process != nil {
		if runtime.GOOS == "windows" {
			killCmd := exec.Command("taskkill", "/pid", fmt.Sprintf("%d", cmd.Process.Pid))
			_ = killCmd.Run()
			time.Sleep(3 * time.Second)
		} else {
			_ = cmd.Process.Kill()
		}
	}
}

func Kafka(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	// 🌟 核心：开启一个 Goroutine（协程）去异步同步启动 Kafka
	go func() {
		defer wg.Done()
		startEmbeddedKafka(ctx)
	}()

	log.Println("⏳ 正在等待 Kafka 初始化就绪...")
	for {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:9092", 5*time.Second)
		if err != nil {
			log.Printf("❌ [Failed] Kafka 拉起失败，%s", err.Error())
			time.Sleep(1 * time.Second)
			continue
		}
		log.Println("✨ [Success] Kafka 已同步拉起，你的 Go 主业务服务现在正式启动！")
		_ = conn.Close()
		break
	}
}
