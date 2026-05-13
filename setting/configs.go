package setting

type Configs struct {
	MySQL  MySQL  `json:"mysql" yaml:"mysql"`
	JWT    JWT    `json:"jwt" yaml:"jwt"`
	Redis  Redis  `json:"redis" yaml:"redis"`
	Minio  Minio  `json:"minio" yaml:"minio"`
	System System `json:"system" yaml:"system"`
}
