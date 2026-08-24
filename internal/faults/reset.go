package faults

import (
	"errors"
	"net/http"

	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

var ErrHijackingUnsupported = errors.New("connection reset simulation requires HTTP/1.x hijacking")

func (e *Engine) applyReset(rule rules.Rule, w http.ResponseWriter, r *http.Request) (bool, error) {
	probability := 1.0
	if rule.Effects.Reset.Probability != nil {
		probability = *rule.Effects.Reset.Probability
	}
	if !e.shouldFail(probability) {
		return false, nil
	}
	e.logFault(rule, r, "reset")
	e.recordFault()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return true, ErrHijackingUnsupported
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		return true, err
	}
	return true, conn.Close()
}
