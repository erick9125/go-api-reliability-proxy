package config

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

func Validate(cfg Config) error {
	if cfg.Version != 1 {
		return fmt.Errorf("invalid configuration: version must be 1")
	}
	if cfg.Proxy.Target == "" {
		return fmt.Errorf("invalid configuration: proxy.target is required")
	}
	if err := validateTarget(cfg.Proxy.Target); err != nil {
		return err
	}
	if cfg.Proxy.Listen == "" {
		return fmt.Errorf("invalid configuration: proxy.listen is required")
	}

	names := make(map[string]struct{}, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		if err := validateRule(rule, names); err != nil {
			return err
		}
	}
	return nil
}

func validateTarget(target string) error {
	parsed, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid configuration: proxy.target is not a valid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid configuration: proxy.target must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("invalid configuration: proxy.target must include a host")
	}
	return nil
}

func validateRule(rule rules.Rule, names map[string]struct{}) error {
	if rule.Name == "" {
		return fmt.Errorf("invalid configuration: rule name is required")
	}
	if _, exists := names[rule.Name]; exists {
		return fmt.Errorf("invalid configuration: duplicate rule name %q", rule.Name)
	}
	names[rule.Name] = struct{}{}

	if rule.Match.Path == "" {
		return ruleError(rule.Name, "match.path is required")
	}
	if !strings.HasPrefix(rule.Match.Path, "/") {
		return ruleError(rule.Name, "match.path must start with /")
	}
	if strings.Contains(rule.Match.Path, "*") {
		if strings.Count(rule.Match.Path, "*") != 1 || !strings.HasSuffix(rule.Match.Path, "/*") {
			return ruleError(rule.Name, "match.path only supports a trailing /* wildcard")
		}
	}
	for _, method := range rule.Match.Methods {
		if !validMethod(method) {
			return ruleError(rule.Name, fmt.Sprintf("invalid HTTP method %q", method))
		}
	}

	effects := 0
	if rule.Effects.Latency != nil {
		effects++
		if err := validateLatency(rule.Name, rule.Effects.Latency); err != nil {
			return err
		}
	}
	if rule.Effects.Failure != nil {
		effects++
		if err := validateFailure(rule.Name, rule.Effects.Failure); err != nil {
			return err
		}
	}
	if rule.Effects.Response != nil {
		effects++
		if err := validateResponse(rule.Name, rule.Effects.Response); err != nil {
			return err
		}
	}
	if rule.Effects.Timeout != nil {
		effects++
		if rule.Effects.Timeout.Duration.Duration < 0 {
			return ruleError(rule.Name, "timeout.duration must be >= 0")
		}
	}
	if rule.Effects.Reset != nil {
		effects++
		if rule.Effects.Reset.Probability != nil {
			if err := validateProbability(rule.Name, "reset.probability", *rule.Effects.Reset.Probability); err != nil {
				return err
			}
		}
	}
	if effects == 0 {
		return ruleError(rule.Name, "at least one effect is required")
	}
	return nil
}

func validateLatency(name string, latency *rules.LatencyConfig) error {
	hasFixed := latency.Fixed != nil
	hasMin := latency.Min != nil
	hasMax := latency.Max != nil
	if hasFixed && (hasMin || hasMax) {
		return ruleError(name, "latency cannot combine fixed with min/max")
	}
	if !hasFixed && (!hasMin || !hasMax) {
		return ruleError(name, "latency requires either fixed or both min and max")
	}
	if hasFixed && latency.Fixed.Duration < 0 {
		return ruleError(name, "latency.fixed must be >= 0")
	}
	if hasMin && latency.Min.Duration < 0 {
		return ruleError(name, "latency.min must be >= 0")
	}
	if hasMax && latency.Max.Duration < 0 {
		return ruleError(name, "latency.max must be >= 0")
	}
	if hasMin && hasMax && latency.Min.Duration > latency.Max.Duration {
		return ruleError(name, "latency.min must be <= latency.max")
	}
	return nil
}

func validateFailure(name string, failure *rules.FailureConfig) error {
	if failure.Probability == nil {
		return ruleError(name, "failure.probability is required")
	}
	if err := validateProbability(name, "failure.probability", *failure.Probability); err != nil {
		return err
	}
	return validateStatus(name, "failure.status", failure.Status)
}

func validateResponse(name string, response *rules.ResponseConfig) error {
	return validateStatus(name, "response.status", response.Status)
}

func validateStatus(name, field string, status int) error {
	if status < 100 || status > 599 {
		return ruleError(name, fmt.Sprintf("%s must be a valid HTTP status between 100 and 599", field))
	}
	return nil
}

func validateProbability(name, field string, value float64) error {
	if value < 0 || value > 1 {
		return ruleError(name, fmt.Sprintf("%s must be between 0 and 1", field))
	}
	return nil
}

func ruleError(name, message string) error {
	return fmt.Errorf("invalid configuration: rule %q: %s", name, message)
}

func validMethod(method string) bool {
	if method == "" {
		return false
	}
	for _, r := range method {
		if !unicode.IsUpper(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
