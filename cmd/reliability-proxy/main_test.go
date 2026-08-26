package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--version"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	if !strings.HasPrefix(got, "reliability-proxy v0.1.0") {
		t.Fatalf("version output %q", got)
	}
}

func TestHelpFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("help output %q", stderr.String())
	}
}

// usageText is hand-written, so nothing stops a new flag from being added
// without documenting it. This keeps the two in step.
func TestUsageTextCoversEveryFlag(t *testing.T) {
	fs, _ := newFlagSet(io.Discard)
	fs.VisitAll(func(f *flag.Flag) {
		if !strings.Contains(usageText, "--"+f.Name) {
			t.Errorf("flag --%s is not documented in usageText", f.Name)
		}
	})
}

func TestConfigFileIsLoaded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	contents := `
version: 1
proxy:
  target: "http://localhost:3000"
rules:
  - name: rate-limit
    match:
      path: "/users/*"
    effects:
      response:
        status: 429
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	// A bad listen address fails after the config was read, which proves the
	// file was parsed and accepted without leaving a server running.
	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", path, "--listen", "256.256.256.256:99999"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected the run to fail on the invalid listen address")
	}
	if strings.Contains(err.Error(), "configuration") {
		t.Fatalf("configuration was rejected: %v", err)
	}
}

func TestConfigFileErrorsSurface(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", path}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error for an empty configuration file")
	}
	if !strings.Contains(err.Error(), "is empty") {
		t.Fatalf("error %q should say the file is empty", err.Error())
	}
}

func TestMissingConfigFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", filepath.Join(t.TempDir(), "nope.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error for a missing configuration file")
	}
	if !strings.Contains(err.Error(), "open configuration file") {
		t.Fatalf("error %q", err.Error())
	}
}

func TestMissingTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when target is missing")
	}
	if !strings.Contains(err.Error(), "proxy.target is required") {
		t.Fatalf("error %q", err.Error())
	}
}
