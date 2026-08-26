package faults

import (
	"context"
	"net/http"

	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

// applyTimeout reports whether the 504 was actually served. If the client gave
// up during the wait, no fault reached it.
func (e *Engine) applyTimeout(ctx context.Context, rule rules.Rule, w http.ResponseWriter, r *http.Request) (bool, error) {
	if err := e.sleeper.Sleep(ctx, rule.Effects.Timeout.Duration.Duration); err != nil {
		return false, err
	}
	writeResponse(w, http.StatusGatewayTimeout, nil, "")
	e.logFault(rule, r, "timeout")
	e.recordFault()
	return true, nil
}
