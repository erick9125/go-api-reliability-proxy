package rules

type Rule struct {
	Name    string      `yaml:"name"`
	Match   MatchConfig `yaml:"match"`
	Effects Effects     `yaml:"effects"`
}

type MatchConfig struct {
	Path    string   `yaml:"path"`
	Methods []string `yaml:"methods"`
}

type Effects struct {
	Latency  *LatencyConfig  `yaml:"latency,omitempty"`
	Failure  *FailureConfig  `yaml:"failure,omitempty"`
	Response *ResponseConfig `yaml:"response,omitempty"`
	Timeout  *TimeoutConfig  `yaml:"timeout,omitempty"`
	Reset    *ResetConfig    `yaml:"reset,omitempty"`
}

type LatencyConfig struct {
	Fixed *Duration `yaml:"fixed,omitempty"`
	Min   *Duration `yaml:"min,omitempty"`
	Max   *Duration `yaml:"max,omitempty"`
}

type FailureConfig struct {
	Probability *float64          `yaml:"probability"`
	Status      int               `yaml:"status"`
	Headers     map[string]string `yaml:"headers,omitempty"`
	Body        string            `yaml:"body,omitempty"`
}

type ResponseConfig struct {
	Status  int               `yaml:"status"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
}

type TimeoutConfig struct {
	Duration Duration `yaml:"duration"`
}

type ResetConfig struct {
	Probability *float64 `yaml:"probability,omitempty"`
}
