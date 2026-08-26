package config

import (
	"fmt"
	"net/url"
	"strings"

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
	return validateEffectReachability(rule)
}

// validateEffectReachability rejects rules whose effects can never run. The
// engine applies effects in a fixed order (latency, timeout, reset, failure,
// response) and the first one that ends the request wins, so without this check
// a rule is accepted, starts silently, and does something other than it says.
//
// Only effects that *always* end the request shadow the ones behind them:
// latency never stops, and a probabilistic reset or failure leaves a reachable
// path, so those combinations stay valid.
func validateEffectReachability(rule rules.Rule) error {
	afterReset := effectsAfterReset(rule)

	shadowedByTimeout := afterReset
	if rule.Effects.Reset != nil {
		shadowedByTimeout = append([]string{"reset"}, afterReset...)
	}
	if rule.Effects.Timeout != nil && len(shadowedByTimeout) > 0 {
		return ruleError(rule.Name, fmt.Sprintf(
			"timeout always ends the request, so %s can never run; split them into separate rules",
			joinEffects(shadowedByTimeout)))
	}

	if rule.Effects.Reset != nil && alwaysResets(rule.Effects.Reset) && len(afterReset) > 0 {
		return ruleError(rule.Name, fmt.Sprintf(
			"reset always ends the request, so %s can never run; set reset.probability below 1 or split them into separate rules",
			joinEffects(afterReset)))
	}
	return nil
}

// effectsAfterReset lists the effects the engine evaluates once reset declined
// to fire, in engine order.
func effectsAfterReset(rule rules.Rule) []string {
	var names []string
	if rule.Effects.Failure != nil {
		names = append(names, "failure")
	}
	if rule.Effects.Response != nil {
		names = append(names, "response")
	}
	return names
}

func alwaysResets(cfg *rules.ResetConfig) bool {
	return cfg.Probability == nil || *cfg.Probability >= 1
}

func joinEffects(names []string) string {
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
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
	if err := validateStatus(name, "failure.status", failure.Status); err != nil {
		return err
	}
	return validateHeaders(name, "failure.headers", failure.Headers)
}

func validateResponse(name string, response *rules.ResponseConfig) error {
	if err := validateStatus(name, "response.status", response.Status); err != nil {
		return err
	}
	return validateHeaders(name, "response.headers", response.Headers)
}

// reservedHeaders are managed by net/http per connection. Setting them from a
// rule produces a malformed response instead of the fault the author intended.
var reservedHeaders = map[string]string{
	"connection":          "hop-by-hop, managed by the server",
	"keep-alive":          "hop-by-hop, managed by the server",
	"proxy-authenticate":  "hop-by-hop, managed by the server",
	"proxy-authorization": "hop-by-hop, managed by the server",
	"te":                  "hop-by-hop, managed by the server",
	"trailer":             "hop-by-hop, managed by the server",
	"transfer-encoding":   "hop-by-hop, managed by the server",
	"upgrade":             "hop-by-hop, managed by the server",
	"content-length":      "computed from the body",
}

func validateHeaders(name, field string, headers map[string]rules.HeaderValues) error {
	for key, values := range headers {
		if strings.TrimSpace(key) == "" {
			return ruleError(name, fmt.Sprintf("%s contains an empty header name", field))
		}
		if reason, reserved := reservedHeaders[strings.ToLower(strings.TrimSpace(key))]; reserved {
			return ruleError(name, fmt.Sprintf("%s must not set %q: %s", field, key, reason))
		}
		if len(values) == 0 {
			return ruleError(name, fmt.Sprintf("%s: %q needs at least one value", field, key))
		}
	}
	return nil
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

// validMethod accepts any RFC 7230 token. The previous rule leaned on
// unicode.IsUpper, which let non-ASCII letters through while rejecting real
// methods that contain a hyphen, such as M-SEARCH.
func validMethod(method string) bool {
	if method == "" {
		return false
	}
	for _, r := range method {
		if !isTokenChar(r) {
			return false
		}
	}
	return true
}

// isTokenChar reports whether r is a tchar from RFC 7230 section 3.2.6.
func isTokenChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune("!#$%&'*+-.^_`|~", r)
}
