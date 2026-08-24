package rules

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDurationUnmarshalYAML(t *testing.T) {
	var d Duration
	if err := yaml.Unmarshal([]byte("1500ms"), &d); err != nil {
		t.Fatal(err)
	}
	if d.Duration != 1500*time.Millisecond {
		t.Fatalf("got %s, want 1.5s", d.Duration)
	}
}

func TestDurationUnmarshalYAMLInvalid(t *testing.T) {
	var d Duration
	err := yaml.Unmarshal([]byte("not-a-duration"), &d)
	if err == nil {
		t.Fatal("expected error")
	}
}
