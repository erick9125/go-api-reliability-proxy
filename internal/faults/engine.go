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
	// Stop reports whether the effect already answered the client, so the
	// request must not be forwarded upstream.
	Stop bool
	// Faulted reports whether at least one effect actually ran. It stays false
	// when a probabilistic effect declined to fire, or when an effect was cut
	// short before taking hold.
	Faulted bool
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
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	random := opts.Random
	if random == nil {
		random = NewLockedRandom(opts.Seed)
	} else if opts.Seed != nil {
		// Both were set, and Random wins. Saying so beats letting the seed look
		// like it took effect.
		logger.Warn("ignoring seed because an explicit random source was provided")
	}
	sleeper := opts.Sleeper
	if sleeper == nil {
		sleeper = TimerSleeper{}
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
	faulted := false

	if rule.Effects.Latency != nil {
		applied, err := e.applyLatency(ctx, rule, r)
		faulted = faulted || applied
		if err != nil {
			return Result{Stop: true, Faulted: faulted}, err
		}
	}
	if rule.Effects.Timeout != nil {
		applied, err := e.applyTimeout(ctx, rule, w, r)
		faulted = faulted || applied
		if err != nil {
			return Result{Stop: true, Faulted: faulted}, err
		}
		return Result{Stop: true, Faulted: faulted}, nil
	}
	if rule.Effects.Reset != nil {
		stop, applied, err := e.applyReset(rule, w, r)
		faulted = faulted || applied
		if err != nil {
			return Result{Stop: true, Faulted: faulted}, err
		}
		if stop {
			return Result{Stop: true, Faulted: faulted}, nil
		}
	}
	if rule.Effects.Failure != nil {
		if e.shouldFail(*rule.Effects.Failure.Probability) {
			e.applyFailure(rule, w, r)
			return Result{Stop: true, Faulted: true}, nil
		}
	}
	if rule.Effects.Response != nil {
		e.applyResponse(rule, w, r)
		return Result{Stop: true, Faulted: true}, nil
	}
	return Result{Stop: false, Faulted: faulted}, nil
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
