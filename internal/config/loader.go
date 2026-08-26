package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load builds the configuration the proxy runs with. An empty path yields the
// defaults, so --target alone is a valid invocation.
//
// Normalizing and validating happen here on purpose: matching relies on rules
// being normalized (methods are compared uppercased), and Validate expects a
// normalized Config. Leaving those steps to the caller made a half-built Config
// representable, and the natural Load + proxy.New path silently skipped them.
func Load(path string, o Overrides) (Config, error) {
	cfg := Default()
	if path != "" {
		parsed, err := decodeFile(path)
		if err != nil {
			return Config{}, err
		}
		cfg = parsed
	}

	cfg.ApplyOverrides(o)
	cfg.Normalize()
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func decodeFile(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open configuration file: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		// An empty file surfaces as a bare EOF, which tells the user nothing.
		if errors.Is(err, io.EOF) {
			return Config{}, fmt.Errorf("parse configuration file: %s is empty", path)
		}
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
		// Blank entries are kept so Validate can reject them. Dropping them left
		// an empty list, which the matcher reads as "every method" — the exact
		// opposite of the restriction the author was trying to express.
		methods := make([]string, 0, len(c.Rules[i].Match.Methods))
		for _, method := range c.Rules[i].Match.Methods {
			methods = append(methods, strings.ToUpper(strings.TrimSpace(method)))
		}
		c.Rules[i].Match.Methods = methods
	}
}
