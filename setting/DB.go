package setting

type MySQL struct {
	Type        string `json:"type" yaml:"type"`
	User        string `json:"user" yaml:"user"`
	Password    string `json:"password" yaml:"password"`
	Host        string `json:"host" yaml:"host"`
	Name        string `json:"name" yaml:"name"`
	MaxIdleCons int    `json:"max-idle-cons" yaml:"max-idle-cons" mapstructure:"max-idle-cons"`
	MaxOpenCons int    `json:"max-open-cons" yaml:"max-open-cons" mapstructure:"max-open-cons"`
}

func (m MySQL) Dsn() string {
	return m.User + ":" + m.Password + "@tcp(" + m.Host + ")/" + m.Name + "?charset=utf8mb4&parseTime=True&loc=Local"
}
