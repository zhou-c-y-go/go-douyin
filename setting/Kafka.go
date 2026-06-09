package setting

type Kafka struct {
	KafkaDir   string `yaml:"kafka-dir" mapstructure:"kafka-dir" json:"kafka-dir"`
	BatPath    string `yaml:"bat-path" mapstructure:"bat-path" json:"bat-path"`
	ConfigPath string `yaml:"config-path" mapstructure:"config-path" json:"config-path"`
}
