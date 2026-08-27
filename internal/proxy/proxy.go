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
	// Faults configures the fault engine; Logger and Metrics above are
	// propagated into it, so setting them here has no effect.
	Faults faults.Options
}

type Handler struct {
	proxy   http.Handler
	matcher *rules.RuleMatcher
	engine  *faults.Engine
	metrics *metrics.Metrics
	logger  *slog.Logger
}

func New(cfg config.Config, opts Options) (*Handler, error) {
	// config.Load guarantees this, but New also takes hand-built Configs.
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
	faultOpts := opts.Faults
	faultOpts.Logger = logger
	faultOpts.Metrics = m

	return &Handler{
		proxy:   newReverseProxy(target, logger),
		matcher: rules.NewMatcher(cfg.Rules),
		engine:  faults.New(faultOpts),
		metrics: m,
		logger:  logger,
	}, nil
}

func (h *Handler) Metrics() *metrics.Metrics {
	return h.metrics
}
