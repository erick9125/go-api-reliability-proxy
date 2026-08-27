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
	Probability *float64                `yaml:"probability"`
	Status      int                     `yaml:"status"`
	Headers     map[string]HeaderValues `yaml:"headers,omitempty"`
	Body        string                  `yaml:"body,omitempty"`
}

type ResponseConfig struct {
	Status  int                     `yaml:"status"`
	Headers map[string]HeaderValues `yaml:"headers,omitempty"`
	Body    string                  `yaml:"body,omitempty"`
}

type TimeoutConfig struct {
	Duration Duration `yaml:"duration"`
}

type ResetConfig struct {
	Probability *float64 `yaml:"probability,omitempty"`
}

// cloneRule returns a rule that shares nothing with the original.
// Adding a field to any effect means adding it here too.
func cloneRule(r Rule) Rule {
	out := r
	out.Match.Methods = cloneStrings(r.Match.Methods)

	if r.Effects.Latency != nil {
		latency := *r.Effects.Latency
		latency.Fixed = cloneDuration(r.Effects.Latency.Fixed)
		latency.Min = cloneDuration(r.Effects.Latency.Min)
		latency.Max = cloneDuration(r.Effects.Latency.Max)
		out.Effects.Latency = &latency
	}
	if r.Effects.Failure != nil {
		failure := *r.Effects.Failure
		failure.Probability = cloneFloat(r.Effects.Failure.Probability)
		failure.Headers = cloneHeaders(r.Effects.Failure.Headers)
		out.Effects.Failure = &failure
	}
	if r.Effects.Response != nil {
		response := *r.Effects.Response
		response.Headers = cloneHeaders(r.Effects.Response.Headers)
		out.Effects.Response = &response
	}
	if r.Effects.Timeout != nil {
		timeout := *r.Effects.Timeout
		out.Effects.Timeout = &timeout
	}
	if r.Effects.Reset != nil {
		reset := *r.Effects.Reset
		reset.Probability = cloneFloat(r.Effects.Reset.Probability)
		out.Effects.Reset = &reset
	}
	return out
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneDuration(in *Duration) *Duration {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneFloat(in *float64) *float64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
