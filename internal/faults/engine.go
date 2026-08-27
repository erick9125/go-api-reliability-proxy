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
	// Stop reports that the effect already answered the client.
	Stop bool
	// Faulted reports that at least one effect actually ran.
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
		// Random wins; say so instead of letting the seed look effective.
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
		stop, applied, err := e.applyReset(ctx, rule, w, r)
		faulted = faulted || applied
		if err != nil {
			return Result{Stop: true, Faulted: faulted}, err
		}
		if stop {
			return Result{Stop: true, Faulted: faulted}, nil
		}
	}
	if rule.Effects.Failure != nil {
		if e.shouldTrigger(*rule.Effects.Failure.Probability) {
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

// shouldTrigger draws for a probabilistic effect. Float64 is [0,1), so a
// probability of 0 never fires and 1 always does.
func (e *Engine) shouldTrigger(probability float64) bool {
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

// latencyDuration picks the delay for one request; the upper bound is exclusive.
func latencyDuration(cfg *rules.LatencyConfig, random Random) time.Duration {
	if cfg.Fixed != nil {
		return cfg.Fixed.Duration
	}
	lo := cfg.Min.Duration
	hi := cfg.Max.Duration
	if hi <= lo {
		return lo
	}
	span := hi - lo
	return lo + time.Duration(float64(span)*random.Float64())
}
