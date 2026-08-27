package faults

import (
	"context"
	"net/http"

	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

// applyLatency reports whether the delay was actually served.
func (e *Engine) applyLatency(ctx context.Context, rule rules.Rule, r *http.Request) (bool, error) {
	delay := latencyDuration(rule.Effects.Latency, e.random)
	if err := e.sleeper.Sleep(ctx, delay); err != nil {
		return false, err
	}
	e.logFault(rule, r, "latency")
	e.metrics.RecordFault()
	return true, nil
}
