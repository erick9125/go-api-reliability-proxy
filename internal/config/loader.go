package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open configuration file: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse configuration file: %w", err)
	}
	return cfg, nil
}

func (c *Config) Normalize() {
	if c.Version == 0 {
		c.Version = 1
	}
	c.Proxy.Listen = strings.TrimSpace(c.Proxy.Listen)
	c.Proxy.Target = strings.TrimSpace(c.Proxy.Target)
	if c.Proxy.Listen == "" {
		c.Proxy.Listen = DefaultListen
	}
	for i := range c.Rules {
		c.Rules[i].Name = strings.TrimSpace(c.Rules[i].Name)
		c.Rules[i].Match.Path = strings.TrimSpace(c.Rules[i].Match.Path)
		methods := make([]string, 0, len(c.Rules[i].Match.Methods))
		for _, method := range c.Rules[i].Match.Methods {
			method = strings.ToUpper(strings.TrimSpace(method))
			if method == "" {
				continue
			}
			methods = append(methods, method)
		}
		c.Rules[i].Match.Methods = methods
	}
}
