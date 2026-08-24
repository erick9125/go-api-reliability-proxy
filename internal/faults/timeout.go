package faults

import (
	"context"
	"net/http"

	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

func (e *Engine) applyTimeout(ctx context.Context, rule rules.Rule, w http.ResponseWriter, r *http.Request) error {
	e.logFault(rule, r, "timeout")
	e.recordFault()
	if err := e.sleeper.Sleep(ctx, rule.Effects.Timeout.Duration.Duration); err != nil {
		return err
	}
	writeResponse(w, http.StatusGatewayTimeout, nil, "")
	return nil
}
