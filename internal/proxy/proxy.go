package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/erick9125/go-api-reliability-proxy/internal/config"
	"github.com/erick9125/go-api-reliability-proxy/internal/faults"
	"github.com/erick9125/go-api-reliability-proxy/internal/metrics"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

type Options struct {
	Logger  *slog.Logger
	Metrics *metrics.Metrics
	Random  faults.Random
	Sleeper faults.Sleeper
	Seed    *int64
}

type Handler struct {
	proxy   http.Handler
	matcher rules.Matcher
	engine  *faults.Engine
	metrics *metrics.Metrics
	logger  *slog.Logger
	rules   int
}

func New(cfg config.Config, opts Options) (*Handler, error) {
	// config.Load already guarantees this, but New is reachable with a Config
	// built by hand. Failing here beats proxying to an empty URL.
	target, err := url.Parse(cfg.Proxy.Target)
	if err != nil {
		return nil, fmt.Errorf("proxy target %q is not a valid URL: %w", cfg.Proxy.Target, err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("proxy target %q must use http or https", cfg.Proxy.Target)
	}
	if target.Host == "" {
		return nil, fmt.Errorf("proxy target %q must include a host", cfg.Proxy.Target)
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	m := opts.Metrics
	if m == nil {
		m = metrics.New()
	}
	engine := faults.New(faults.Options{
		Random:  opts.Random,
		Sleeper: opts.Sleeper,
		Seed:    opts.Seed,
		Metrics: m,
		Logger:  logger,
	})
	return &Handler{
		proxy:   newReverseProxy(target, logger),
		matcher: rules.NewMatcher(cfg.Rules),
		engine:  engine,
		metrics: m,
		logger:  logger,
		rules:   len(cfg.Rules),
	}, nil
}

func (h *Handler) Metrics() *metrics.Metrics {
	return h.metrics
}

func (h *Handler) RuleCount() int {
	return h.rules
}
