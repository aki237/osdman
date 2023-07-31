package config

type Config struct {
	Domains map[string]Domain `yaml:"domains"`
}

type Domain struct {
	StatCmd string          `yaml:"stat_cmd"`
	Verbs   map[string]Verb `yaml:"verbs"`
}

type Verb struct {
	Command string `yaml:"cmd"`
}
