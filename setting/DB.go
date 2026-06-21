package setting

import "fmt"

type SQL struct {
	Type        string `json:"type" yaml:"type"`
	User        string `json:"user" yaml:"user"`
	Password    string `json:"password" yaml:"password"`
	Host        string `json:"host" yaml:"host"`
	Name        string `json:"name" yaml:"name"`
	MaxIdleCons int    `json:"max-idle-cons" yaml:"max-idle-cons" mapstructure:"max-idle-cons"`
	MaxOpenCons int    `json:"max-open-cons" yaml:"max-open-cons" mapstructure:"max-open-cons"`
}

// MysqlDSN 提取专属于 MySQL 的 DSN 规则
func (m SQL) MysqlDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		m.User, m.Password, m.Host, m.Name)
}

// PostgresDSN 提取专属于 PostgreSQL 的 DSN 规则
func (m SQL) PostgresDSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		m.Host, m.User, m.Password, m.Name)
}
