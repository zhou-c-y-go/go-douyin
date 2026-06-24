package Init

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres" // 💡 动态引入多驱动支持
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

// InitDatabaseFactory 工业级数据源动态孵化工厂
func InitDatabaseFactory() *gorm.DB {
	m := global.GLOB_CONFIG.SQL
	var dialector gorm.Dialector

	// 1. 🎯 【解耦核心】根据配置动态判定底层驱动品牌，核心业务层完全不感知
	switch m.Type {
	case "mysql":
		dialector = mysql.New(mysql.Config{
			DSN: m.MysqlDSN(),
		})
	case "postgres":
		// 如果以后更换成 pg，只需解开此处的驱动包引入，配置一换瞬间并网
		dialector = postgres.Open(m.PostgresDSN())
	default:
		panic(fmt.Sprintf("❌ [DB-Init] 不支持的数据库驱动类型: %s", m.Type))
	}

	// 2. 优雅的自愈重连接链
	var db *gorm.DB
	var err error
	const maxRetries = 10

	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(dialector, &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info), // 生产环境可改为 logger.Error
		})
		if err == nil {
			break
		}
		fmt.Printf("⚠️  [DB-Retry] 等待数据库基础设施启动 (%d/%d): %v\n", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		panic("💥 [DB-Fatal] 数据库连接彻底失败，熔断阻断启动: " + err.Error())
	}

	// 3. 连接池调优性能并网
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.SetMaxIdleConns(m.MaxIdleCons)
		sqlDB.SetMaxOpenConns(m.MaxOpenCons)
		sqlDB.SetConnMaxLifetime(time.Hour * 2) // 工业级规范：必须加上连接生命周期控制
	}

	fmt.Printf("✅ [DB-Success] %s 数据库连接建立成功，连接池并网完毕\n", m.Type)
	return db
}

// RegisterAutoMigrateTable ── 职责解耦：将极其沉重的建表、更表动作单独隔离
func RegisterAutoMigrateTable(db *gorm.DB) {
	if global.GLOB_CONFIG.SQL.Type == "mysql" {
		db.Set("gorm:table_options", "ENGINE=InnoDB")
	}

	fmt.Println("🔄 [DB-Migrate] 正在全力回源执行全量表结构一致性同步...")
	err := db.AutoMigrate(
		&pojo.User{},
		&pojo.Admin{},
		&pojo.Video{},
		&pojo.Comment{},
		&pojo.LikeRecord{},
		&pojo.FavoriteRecord{},
		&pojo.FollowRelation{},
	)
	if err != nil {
		panic(fmt.Sprintf("❌ [DB-Migrate-Fatal] 自动刷表进化失败: %v", err))
	}
	fmt.Println("🎉 [DB-Migrate-Success] 全量结构表自愈更新大功告成！")
}
