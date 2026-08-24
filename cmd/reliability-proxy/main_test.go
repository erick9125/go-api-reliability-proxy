package main

import (
	"bytes"
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
