package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erick9125/go-api-reliability-proxy/internal/config"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
	"gopkg.in/yaml.v3"
)

func TestLoadValidConfig(t *testing.T) {
	path := writeConfig(t, `
version: 1
proxy:
  listen: ":8080"
  target: "http://localhost:3000"
rules:
  - name: payments-latency
    match:
      path: "/payments/*"
      methods:
        - POST
    effects:
      latency:
        fixed: 1200ms
`)
	cfg, err := config.Load(path, config.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Proxy.Target != "http://localhost:3000" {
		t.Fatalf("unexpected target %q", cfg.Proxy.Target)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("got %d rules", len(cfg.Rules))
	}
	if cfg.Rules[0].Effects.Latency == nil || cfg.Rules[0].Effects.Latency.Fixed == nil {
		t.Fatal("expected fixed latency")
	}
}

func TestValidateRejectsInvalidConfigs(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantSub string
	}{
		{
			name: "missing target",
			yaml: `
version: 1
proxy:
  listen: ":8080"
`,
			wantSub: "proxy.target is required",
		},
		{
			name: "invalid target",
			yaml: `
version: 1
proxy:
  target: "ftp://localhost:3000"
`,
			wantSub: "must use http or https",
		},
		{
			name: "invalid probability",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: inventory-failure
    match:
      path: "/inventory/*"
    effects:
      failure:
        probability: 1.5
        status: 503
`,
			wantSub: `rule "inventory-failure": failure.probability must be between 0 and 1`,
		},
		{
			name: "invalid duration",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: slow
    match:
      path: "/slow"
    effects:
      latency:
        fixed: not-a-duration
`,
			wantSub: "invalid duration",
		},
		{
			name: "min greater than max",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: jitter
    match:
      path: "/jitter"
    effects:
      latency:
        min: 2s
        max: 1s
`,
			wantSub: "latency.min must be <= latency.max",
		},
		{
			name: "invalid HTTP status",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: bad-status
    match:
      path: "/x"
    effects:
      response:
        status: 999
`,
			wantSub: "valid HTTP status",
		},
		{
			name: "duplicate rule name",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: same
    match:
      path: "/a"
    effects:
      response:
        status: 503
  - name: same
    match:
      path: "/b"
    effects:
      response:
        status: 500
`,
			wantSub: "duplicate rule name",
		},
		{
			name: "missing rule name",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - match:
      path: "/a"
    effects:
      response:
        status: 503
`,
			wantSub: "rule name is required",
		},
		{
			name: "timeout shadows response",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: timeout-and-response
    match:
      path: "/a"
    effects:
      timeout:
        duration: 30s
      response:
        status: 429
`,
			wantSub: "timeout always ends the request, so response can never run",
		},
		{
			name: "timeout shadows reset failure and response",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: timeout-and-everything
    match:
      path: "/a"
    effects:
      timeout:
        duration: 30s
      reset: {}
      failure:
        probability: 0.5
        status: 503
      response:
        status: 429
`,
			wantSub: "so reset, failure and response can never run",
		},
		{
			name: "certain reset shadows failure",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: reset-and-failure
    match:
      path: "/a"
    effects:
      reset: {}
      failure:
        probability: 0.5
        status: 503
`,
			wantSub: "reset always ends the request, so failure can never run",
		},
		{
			name: "reset with probability 1 shadows response",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: certain-reset-and-response
    match:
      path: "/a"
    effects:
      reset:
        probability: 1.0
      response:
        status: 429
`,
			wantSub: "reset always ends the request, so response can never run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "invalid duration" {
				var cfg config.Config
				err := yaml.Unmarshal([]byte(tt.yaml), &cfg)
				if err == nil {
					t.Fatal("expected unmarshal error")
				}
				if !strings.Contains(err.Error(), tt.wantSub) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantSub)
				}
				return
			}
			var cfg config.Config
			if err := yaml.Unmarshal([]byte(tt.yaml), &cfg); err != nil {
				t.Fatal(err)
			}
			cfg.Normalize()
			err := config.Validate(cfg)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

// These combinations have a reachable path and must stay valid.
func TestValidateAcceptsReachableEffectCombinations(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			// response covers the case where the probabilistic failure did not fire.
			name: "failure then response",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: flaky-then-rate-limited
    match:
      path: "/a"
    effects:
      failure:
        probability: 0.2
        status: 503
      response:
        status: 429
`,
		},
		{
			// A reset that does not always fire leaves failure reachable.
			name: "probabilistic reset then failure",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: sometimes-reset
    match:
      path: "/a"
    effects:
      reset:
        probability: 0.3
      failure:
        probability: 0.5
        status: 503
`,
		},
		{
			// Latency never ends the request.
			name: "latency then failure",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: slow-and-flaky
    match:
      path: "/a"
    effects:
      latency:
        fixed: 100ms
      failure:
        probability: 0.5
        status: 503
`,
		},
		{
			name: "latency then timeout",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: slow-then-timeout
    match:
      path: "/a"
    effects:
      latency:
        fixed: 100ms
      timeout:
        duration: 30s
`,
		},
		{
			name: "timeout alone",
			yaml: `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: just-timeout
    match:
      path: "/a"
    effects:
      timeout:
        duration: 30s
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg config.Config
			if err := yaml.Unmarshal([]byte(tt.yaml), &cfg); err != nil {
				t.Fatal(err)
			}
			cfg.Normalize()
			if err := config.Validate(cfg); err != nil {
				t.Fatalf("expected a valid configuration, got %v", err)
			}
		})
	}
}

// Load owns the whole pipeline, so callers cannot skip a step.
func TestLoadNormalizesAndValidates(t *testing.T) {
	path := writeConfig(t, `
proxy:
  target: "http://localhost:3000"
rules:
  - name: users
    match:
      path: "  /users  "
      methods:
        - get
    effects:
      response:
        status: 429
`)
	cfg, err := config.Load(path, config.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 1 {
		t.Fatalf("version = %d, want the default 1", cfg.Version)
	}
	if cfg.Proxy.Listen != config.DefaultListen {
		t.Fatalf("listen = %q, want %q", cfg.Proxy.Listen, config.DefaultListen)
	}
	if got := cfg.Rules[0].Match.Path; got != "/users" {
		t.Fatalf("path = %q, want it trimmed", got)
	}
	if got := cfg.Rules[0].Match.Methods[0]; got != "GET" {
		t.Fatalf("method = %q, want it uppercased", got)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	path := writeConfig(t, `
proxy:
  target: "ftp://localhost:3000"
`)
	if _, err := config.Load(path, config.Overrides{}); err == nil {
		t.Fatal("expected Load to reject an invalid configuration")
	}
}

func TestLoadWithoutPathUsesDefaultsAndOverrides(t *testing.T) {
	cfg, err := config.Load("", config.Overrides{Target: "http://localhost:3000"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Proxy.Listen != config.DefaultListen {
		t.Fatalf("listen = %q", cfg.Proxy.Listen)
	}
	if cfg.Proxy.Target != "http://localhost:3000" {
		t.Fatalf("target = %q", cfg.Proxy.Target)
	}
}

// A blank method must fail at startup, not widen the rule to every method.
func TestBlankMethodIsRejected(t *testing.T) {
	path := writeConfig(t, `
proxy:
  target: "http://localhost:3000"
rules:
  - name: only-post
    match:
      path: "/payments"
      methods:
        - "  "
    effects:
      response:
        status: 503
`)
	_, err := config.Load(path, config.Overrides{})
	if err == nil {
		t.Fatal("expected a blank method to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid HTTP method") {
		t.Fatalf("error %q, want it to mention the invalid method", err.Error())
	}
}

// Omitting the key still means "all methods".
func TestOmittedMethodsStillMeansAll(t *testing.T) {
	path := writeConfig(t, `
proxy:
  target: "http://localhost:3000"
rules:
  - name: everything
    match:
      path: "/payments"
    effects:
      response:
        status: 503
`)
	cfg, err := config.Load(path, config.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules[0].Match.Methods) != 0 {
		t.Fatalf("methods = %#v, want empty", cfg.Rules[0].Match.Methods)
	}
}

func TestMethodValidation(t *testing.T) {
	tests := []struct {
		method string
		valid  bool
	}{
		{method: "GET", valid: true},
		{method: "PATCH", valid: true},
		{method: "M-SEARCH", valid: true}, // RFC 7230 token, was rejected before
		{method: "X_CUSTOM.v2", valid: true},
		{method: "Ä", valid: false}, // unicode.IsUpper used to accept this
		{method: "GET METHOD", valid: false},
		{method: "GET\tX", valid: false},
		{method: "", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			cfg := config.Config{
				Version: 1,
				Proxy:   config.ProxyConfig{Target: "http://localhost:3000"},
				Rules: []rules.Rule{{
					Name:    "rule",
					Match:   rules.MatchConfig{Path: "/a", Methods: []string{tt.method}},
					Effects: rules.Effects{Response: &rules.ResponseConfig{Status: 503}},
				}},
			}
			cfg.Normalize()
			err := config.Validate(cfg)
			if tt.valid && err != nil {
				t.Fatalf("method %q should be valid, got %v", tt.method, err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("method %q should be rejected", tt.method)
			}
		})
	}
}

func TestReservedResponseHeadersAreRejected(t *testing.T) {
	for _, header := range []string{"Content-Length", "Transfer-Encoding", "connection", "Upgrade"} {
		t.Run(header, func(t *testing.T) {
			cfg := config.Config{
				Version: 1,
				Proxy:   config.ProxyConfig{Target: "http://localhost:3000"},
				Rules: []rules.Rule{{
					Name:  "rule",
					Match: rules.MatchConfig{Path: "/a"},
					Effects: rules.Effects{Response: &rules.ResponseConfig{
						Status:  503,
						Headers: map[string]rules.HeaderValues{header: {"1"}},
					}},
				}},
			}
			cfg.Normalize()
			if err := config.Validate(cfg); err == nil {
				t.Fatalf("header %q should be rejected", header)
			}
		})
	}
}

// A scalar and a list must both parse.
func TestHeadersAcceptScalarAndList(t *testing.T) {
	path := writeConfig(t, `
proxy:
  target: "http://localhost:3000"
rules:
  - name: unauthorized
    match:
      path: "/a"
    effects:
      response:
        status: 401
        headers:
          Retry-After: "10"
          WWW-Authenticate:
            - Bearer
            - Basic realm="api"
`)
	cfg, err := config.Load(path, config.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	headers := cfg.Rules[0].Effects.Response.Headers
	if got := headers["Retry-After"]; len(got) != 1 || got[0] != "10" {
		t.Fatalf("Retry-After = %#v, want one value", got)
	}
	if got := headers["WWW-Authenticate"]; len(got) != 2 || got[0] != "Bearer" {
		t.Fatalf("WWW-Authenticate = %#v, want two values", got)
	}
}

func TestCLIOverridesYAML(t *testing.T) {
	cfg := config.Config{
		Version: 1,
		Proxy: config.ProxyConfig{
			Listen: ":8080",
			Target: "http://localhost:3000",
		},
	}
	cfg.ApplyOverrides(config.Overrides{Listen: ":9090"})
	cfg.Normalize()
	if cfg.Proxy.Listen != ":9090" {
		t.Fatalf("got listen %q", cfg.Proxy.Listen)
	}
}

func TestDefaultListen(t *testing.T) {
	cfg := config.Default()
	cfg.Proxy.Target = "http://localhost:3000"
	cfg.Normalize()
	if err := config.Validate(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Proxy.Listen != config.DefaultListen {
		t.Fatalf("got listen %q", cfg.Proxy.Listen)
	}
}

func TestNormalizeUppercasesMethods(t *testing.T) {
	cfg := config.Config{
		Version: 1,
		Proxy: config.ProxyConfig{
			Target: "http://localhost:3000",
		},
		Rules: []rules.Rule{{
			Name: "users",
			Match: rules.MatchConfig{
				Path:    "/users",
				Methods: []string{"get", "post"},
			},
			Effects: rules.Effects{
				Response: &rules.ResponseConfig{Status: 429},
			},
		}},
	}
	cfg.Normalize()
	if err := config.Validate(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Rules[0].Match.Methods[0] != "GET" || cfg.Rules[0].Match.Methods[1] != "POST" {
		t.Fatalf("methods not normalized: %#v", cfg.Rules[0].Match.Methods)
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
