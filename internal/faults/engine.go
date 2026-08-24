package faults

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/erick9125/go-api-reliability-proxy/internal/metrics"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

type Result struct {
	Stop bool
}

type Options struct {
	Random  Random
	Sleeper Sleeper
	Seed    *int64
	Metrics *metrics.Metrics
	Logger  *slog.Logger
}

type Engine struct {
	random  Random
	sleeper Sleeper
	metrics *metrics.Metrics
	logger  *slog.Logger
}

func New(opts Options) *Engine {
	random := opts.Random
	if random == nil {
		random = NewLockedRandom(opts.Seed)
	}
	sleeper := opts.Sleeper
	if sleeper == nil {
		sleeper = TimerSleeper{}
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	m := opts.Metrics
	if m == nil {
		m = metrics.New()
	}
	return &Engine{
		random:  random,
		sleeper: sleeper,
		metrics: m,
		logger:  logger,
	}
}

func (e *Engine) Apply(ctx context.Context, rule rules.Rule, w http.ResponseWriter, r *http.Request) (Result, error) {
	if rule.Effects.Latency != nil {
		if err := e.applyLatency(ctx, rule, r); err != nil {
			return Result{Stop: true}, err
		}
	}
	if rule.Effects.Timeout != nil {
		if err := e.applyTimeout(ctx, rule, w, r); err != nil {
			return Result{Stop: true}, err
		}
		return Result{Stop: true}, nil
	}
	if rule.Effects.Reset != nil {
		stop, err := e.applyReset(rule, w, r)
		if err != nil {
			return Result{Stop: true}, err
		}
		if stop {
			return Result{Stop: true}, nil
		}
	}
	if rule.Effects.Failure != nil {
		if e.shouldFail(*rule.Effects.Failure.Probability) {
			e.applyFailure(rule, w, r)
			return Result{Stop: true}, nil
		}
	}
	if rule.Effects.Response != nil {
		e.applyResponse(rule, w, r)
		return Result{Stop: true}, nil
	}
	return Result{Stop: false}, nil
}

func (e *Engine) shouldFail(probability float64) bool {
	return e.random.Float64() < probability
}

func (e *Engine) logFault(rule rules.Rule, r *http.Request, effect string) {
	e.logger.Info("fault injected",
		"rule", rule.Name,
		"method", r.Method,
		"path", r.URL.Path,
		"effect", effect,
	)
}

func (e *Engine) recordFault() {
	e.metrics.RecordFault()
}

func latencyDuration(cfg *rules.LatencyConfig, random Random) time.Duration {
	if cfg.Fixed != nil {
		return cfg.Fixed.Duration
	}
	min := cfg.Min.Duration
	max := cfg.Max.Duration
	if max <= min {
		return min
	}
	span := max - min
	return min + time.Duration(float64(span)*random.Float64())
}
