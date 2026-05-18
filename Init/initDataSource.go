package Init

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"fmt"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func GormMySQL() *gorm.DB {
	m := global.GLOB_CONFIG.MySQL
	mysqlConfig := mysql.Config{
		DSN: m.Dsn(), // DSN data source name
	}
	if db, err := gorm.Open(mysql.New(mysqlConfig), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}); err != nil {
		return nil
	} else {
		db.Set("gorm:table_options", "ENGINE=InnoDB")
		sqlDB, _ := db.DB()
		sqlDB.SetMaxIdleConns(m.MaxIdleCons)
		sqlDB.SetMaxOpenConns(m.MaxOpenCons)
		fmt.Println("连接成功")
		err := db.AutoMigrate(
			&pojo.User{},
			&pojo.Admin{},
			&pojo.Video{},   // 新增：视频表
			&pojo.Comment{}, // 新增：评论表（带物化路径索引）
		)
		if err != nil {
			fmt.Println("生成表失败")
		}
		return db
	}
}
