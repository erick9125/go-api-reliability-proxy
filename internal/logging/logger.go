package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format selects the handler used for log output.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Options configures the application logger; the zero value is text at info level.
type Options struct {
	Level  slog.Level
	Format Format
}

func New(w io.Writer, opts Options) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	handlerOpts := &slog.HandlerOptions{Level: opts.Level}
	if opts.Format == FormatJSON {
		return slog.New(slog.NewJSONHandler(w, handlerOpts))
	}
	return slog.New(slog.NewTextHandler(w, handlerOpts))
}

// ParseLevel accepts the slog level names, case-insensitively.
func ParseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q: use debug, info, warn or error", value)
	}
}

func ParseFormat(value string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(FormatText):
		return FormatText, nil
	case string(FormatJSON):
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("invalid log format %q: use text or json", value)
	}
}
