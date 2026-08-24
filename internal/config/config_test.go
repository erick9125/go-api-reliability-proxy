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
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Normalize()
	if err := config.Validate(cfg); err != nil {
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
