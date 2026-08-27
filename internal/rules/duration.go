package rules

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return fmt.Errorf("duration must be a string such as 1200ms: %w", err)
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", raw, err)
	}
	d.Duration = parsed
	return nil
}
