package setting

type Kafka struct {
	BootstrapServers string `mapstructure:"bootstrap-servers" json:"bootstrap-servers" yaml:"bootstrap-servers"`

	// 下面这几个是旧字段，Docker 模式下可以不要。
	// 你也可以先保留，避免其他地方编译报错。
	KafkaDir   string `mapstructure:"kafka-dir" json:"kafka-dir" yaml:"kafka-dir"`
	BatPath    string `mapstructure:"bat-path" json:"bat-path" yaml:"bat-path"`
	ConfigPath string `mapstructure:"config-path" json:"config-path" yaml:"config-path"`
}
