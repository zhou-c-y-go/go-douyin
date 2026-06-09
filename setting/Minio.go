package setting

type Minio struct {
	EndPoint        string `yaml:"end-point" mapstructure:"end-point" json:"end-point"`
	AccessKeyID     string `yaml:"access-key-id" mapstructure:"access-key-id" json:"access-key-id"`
	SecretAccessKey string `yaml:"secret-access-key" mapstructure:"secret-access-key" json:"secret-access-key"`
}
