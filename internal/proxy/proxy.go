package proxy

import (
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
	target, err := url.Parse(cfg.Proxy.Target)
	if err != nil {
		return nil, err
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
