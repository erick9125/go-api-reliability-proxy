package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{in: "", want: slog.LevelInfo},
		{in: "info", want: slog.LevelInfo},
		{in: "DEBUG", want: slog.LevelDebug},
		{in: " warn ", want: slog.LevelWarn},
		{in: "warning", want: slog.LevelWarn},
		{in: "error", want: slog.LevelError},
		{in: "verbose", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseLevel(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseLevel(%q) should fail", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	if got, err := ParseFormat("JSON"); err != nil || got != FormatJSON {
		t.Fatalf("ParseFormat(JSON) = %v, %v", got, err)
	}
	if got, err := ParseFormat(""); err != nil || got != FormatText {
		t.Fatalf("ParseFormat(empty) = %v, %v", got, err)
	}
	if _, err := ParseFormat("xml"); err == nil {
		t.Fatal("ParseFormat(xml) should fail")
	}
}

func TestJSONFormatEmitsParseableRecords(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, Options{Format: FormatJSON})
	logger.Info("started", "listen", "127.0.0.1:8080")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, buf.String())
	}
	if record["msg"] != "started" {
		t.Fatalf("msg = %v", record["msg"])
	}
	if record["listen"] != "127.0.0.1:8080" {
		t.Fatalf("listen = %v", record["listen"])
	}
}

func TestLevelFiltersLowerSeverity(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, Options{Level: slog.LevelWarn})
	logger.Info("should be dropped")
	logger.Warn("should be kept")

	out := buf.String()
	if strings.Contains(out, "should be dropped") {
		t.Fatalf("info record survived a warn level: %q", out)
	}
	if !strings.Contains(out, "should be kept") {
		t.Fatalf("warn record was dropped: %q", out)
	}
}

func TestDebugLevelLetsDebugThrough(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, Options{Level: slog.LevelDebug})
	logger.Debug("details")
	if !strings.Contains(buf.String(), "details") {
		t.Fatalf("debug record was dropped: %q", buf.String())
	}
}
