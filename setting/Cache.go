package setting

type Redis struct {
	Addr     string `yaml:"addr" json:"addr"`
	DB       int    `yaml:"db" json:"db"`
	Password string `json:"password" yaml:"password"`
}
