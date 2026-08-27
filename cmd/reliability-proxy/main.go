package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/erick9125/go-api-reliability-proxy/internal/config"
	"github.com/erick9125/go-api-reliability-proxy/internal/faults"
	"github.com/erick9125/go-api-reliability-proxy/internal/logging"
	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
	"github.com/erick9125/go-api-reliability-proxy/internal/server"
)

var (
	version = "0.1.0"
	commit  = "none"
	date    = "unknown"
)

// usageText is curated because the string flags default to "" so overrides can
// tell "not given" apart. TestUsageTextCoversEveryFlag keeps it in sync.
const usageText = `Usage:

  reliability-proxy [options]

Options:

  --config string
      configuration file

  --target string
      upstream API URL

  --listen string
      proxy listen address (falls back to proxy.listen, then to 127.0.0.1:8080)

  --seed int
      RNG seed for repeatable fault injection on sequential traffic

  --log-level string
      debug, info, warn or error (default "info")

  --log-format string
      text or json (default "text")

  --version
      print version and exit
`

func main() {
	// Use the injected stderr, not the global slog default.
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "reliability-proxy: %v\n", err)
		os.Exit(1)
	}
}

type cliFlags struct {
	configPath  *string
	target      *string
	listen      *string
	seed        *int64
	logLevel    *string
	logFormat   *string
	showVersion *bool
}

func newFlagSet(out io.Writer) (*flag.FlagSet, cliFlags) {
	fs := flag.NewFlagSet("reliability-proxy", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Usage = func() {
		_, _ = fmt.Fprint(fs.Output(), usageText)
	}
	return fs, cliFlags{
		configPath:  fs.String("config", "", "configuration file"),
		target:      fs.String("target", "", "upstream API URL"),
		listen:      fs.String("listen", "", "proxy listen address"),
		seed:        fs.Int64("seed", 0, "RNG seed for repeatable fault injection on sequential traffic"),
		logLevel:    fs.String("log-level", "info", "log level: debug, info, warn or error"),
		logFormat:   fs.String("log-format", "text", "log format: text or json"),
		showVersion: fs.Bool("version", false, "print version and exit"),
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs, flags := newFlagSet(stderr)
	configPath, target, listen := flags.configPath, flags.target, flags.listen
	seed, showVersion := flags.seed, flags.showVersion

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if *showVersion {
		_, _ = fmt.Fprintf(stdout, "reliability-proxy %s\n", formatVersion())
		return nil
	}

	cfg, err := config.Load(*configPath, config.Overrides{
		Listen: *listen,
		Target: *target,
	})
	if err != nil {
		return err
	}

	var seedValue *int64
	seedSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "seed" {
			seedSet = true
		}
	})
	if seedSet {
		seedCopy := *seed
		seedValue = &seedCopy
	}

	level, err := logging.ParseLevel(*flags.logLevel)
	if err != nil {
		return err
	}
	format, err := logging.ParseFormat(*flags.logFormat)
	if err != nil {
		return err
	}

	// Logs are diagnostics: stderr. stdout stays for program output.
	logger := logging.New(stderr, logging.Options{Level: level, Format: format})
	handler, err := proxy.New(cfg, proxy.Options{
		Logger: logger,
		Faults: faults.Options{Seed: seedValue},
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return server.Run(ctx, server.Options{
		Addr:    cfg.Proxy.Listen,
		Handler: handler,
		Logger:  logger,
		// Announcing the start before the bind made a failed startup look
		// successful to anything reading the logs.
		OnListen: func(addr net.Addr) {
			logger.Info("reliability proxy started",
				"listen", addr.String(),
				"target", cfg.Proxy.Target,
				"rules", len(cfg.Rules),
				"version", formatVersion(),
				"commit", commit,
				"date", date,
			)
		},
	})
}

func formatVersion() string {
	return "v" + strings.TrimPrefix(version, "v")
}
