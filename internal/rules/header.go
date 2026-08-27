package rules

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// HeaderValues holds every value for one response header. YAML accepts a
// scalar or a list, so single-valued headers keep their plain form.
type HeaderValues []string

func (h *HeaderValues) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var single string
		if err := value.Decode(&single); err != nil {
			return fmt.Errorf("header value must be a string or a list of strings: %w", err)
		}
		*h = HeaderValues{single}
		return nil
	case yaml.SequenceNode:
		var many []string
		if err := value.Decode(&many); err != nil {
			return fmt.Errorf("header value must be a string or a list of strings: %w", err)
		}
		*h = many
		return nil
	default:
		return fmt.Errorf("header value must be a string or a list of strings")
	}
}

func (h HeaderValues) clone() HeaderValues {
	if h == nil {
		return nil
	}
	out := make(HeaderValues, len(h))
	copy(out, h)
	return out
}

func cloneHeaders(in map[string]HeaderValues) map[string]HeaderValues {
	if in == nil {
		return nil
	}
	out := make(map[string]HeaderValues, len(in))
	for k, v := range in {
		out[k] = v.clone()
	}
	return out
}
