package core

import (
	"Go_Project/global"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"os"
)

func Viper() *viper.Viper {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	v := viper.New()
	v.AddConfigPath(path)     //设置读取的文件路径
	v.SetConfigName("config") //设置读取的文件名
	v.SetConfigType("yaml")   //设置文件的类型
	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	v.WatchConfig()

	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("config file changed:", e.Name)
		if err = v.Unmarshal(&global.GLOB_CONFIG); err != nil {
			fmt.Println(err)
		}
	})
	if err = v.Unmarshal(&global.GLOB_CONFIG); err != nil {
		fmt.Println(err)
	}
	return v
}
