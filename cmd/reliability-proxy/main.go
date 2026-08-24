package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/erick9125/go-api-reliability-proxy/internal/config"
	"github.com/erick9125/go-api-reliability-proxy/internal/logging"
	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
	"github.com/erick9125/go-api-reliability-proxy/internal/server"
)

var (
	version = "0.1.0"
	commit  = "none"
	date    = "unknown"
)

const usageText = `Usage:

  reliability-proxy [options]

Options:

  --config string
      configuration file

  --target string
      upstream API URL

  --listen string
      proxy listen address (default "127.0.0.1:8080")

  --seed int
      optional RNG seed for deterministic fault injection

  --version
      print version and exit
`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("reliability-proxy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), usageText)
	}

	configPath := fs.String("config", "", "configuration file")
	target := fs.String("target", "", "upstream API URL")
	listen := fs.String("listen", "", "proxy listen address")
	seed := fs.Int64("seed", 0, "optional RNG seed for deterministic fault injection")
	showVersion := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if *showVersion {
		fmt.Fprintf(stdout, "reliability-proxy %s\n", formatVersion())
		return nil
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	cfg.ApplyOverrides(config.Overrides{
		Listen: *listen,
		Target: *target,
	})
	cfg.Normalize()
	if err := config.Validate(cfg); err != nil {
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

	logger := logging.New(stdout)
	handler, err := proxy.New(cfg, proxy.Options{
		Logger: logger,
		Seed:   seedValue,
	})
	if err != nil {
		return err
	}

	logger.Info("reliability proxy started",
		"listen", cfg.Proxy.Listen,
		"target", cfg.Proxy.Target,
		"rules", len(cfg.Rules),
		"version", formatVersion(),
		"commit", commit,
		"date", date,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return server.Run(ctx, server.Options{
		Addr:    cfg.Proxy.Listen,
		Handler: handler,
		Logger:  logger,
	})
}

func loadConfig(path string) (config.Config, error) {
	if path == "" {
		return config.Default(), nil
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

func formatVersion() string {
	return "v" + strings.TrimPrefix(version, "v")
}
