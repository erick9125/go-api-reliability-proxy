package faults

import (
	"context"
	"net/http"

	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

func (e *Engine) applyLatency(ctx context.Context, rule rules.Rule, r *http.Request) error {
	delay := latencyDuration(rule.Effects.Latency, e.random)
	e.logFault(rule, r, "latency")
	e.recordFault()
	return e.sleeper.Sleep(ctx, delay)
}
