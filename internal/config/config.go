package config

import "github.com/erick9125/go-api-reliability-proxy/internal/rules"

const DefaultListen = "127.0.0.1:8080"

type Config struct {
	Version int          `yaml:"version"`
	Proxy   ProxyConfig  `yaml:"proxy"`
	Rules   []rules.Rule `yaml:"rules"`
}

type ProxyConfig struct {
	Listen string `yaml:"listen"`
	Target string `yaml:"target"`
}

type Overrides struct {
	Listen string
	Target string
}

func Default() Config {
	return Config{
		Version: 1,
		Proxy: ProxyConfig{
			Listen: DefaultListen,
		},
	}
}

func (c *Config) ApplyOverrides(o Overrides) {
	if o.Listen != "" {
		c.Proxy.Listen = o.Listen
	}
	if o.Target != "" {
		c.Proxy.Target = o.Target
	}
}
