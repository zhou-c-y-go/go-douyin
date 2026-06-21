package setting

type Configs struct {
	SQL     SQL     `json:"sql" yaml:"sql"`
	JWT     JWT     `json:"jwt" yaml:"jwt"`
	Redis   Redis   `json:"redis" yaml:"redis"`
	Storage Storage `json:"storage" yaml:"storage"`
	System  System  `json:"system" yaml:"system"`
	MQ      Kafka   `json:"mq" yaml:"mq"`
}
