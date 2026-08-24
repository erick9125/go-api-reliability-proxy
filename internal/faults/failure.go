package faults

import (
	"io"
	"net/http"

	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

func (e *Engine) applyFailure(rule rules.Rule, w http.ResponseWriter, r *http.Request) {
	e.logFault(rule, r, "failure")
	e.recordFault()
	cfg := rule.Effects.Failure
	writeResponse(w, cfg.Status, cfg.Headers, cfg.Body)
}

func (e *Engine) applyResponse(rule rules.Rule, w http.ResponseWriter, r *http.Request) {
	e.logFault(rule, r, "response")
	e.recordFault()
	cfg := rule.Effects.Response
	writeResponse(w, cfg.Status, cfg.Headers, cfg.Body)
}

func writeResponse(w http.ResponseWriter, status int, headers map[string]string, body string) {
	for key, value := range headers {
		w.Header().Set(key, value)
	}
	w.WriteHeader(status)
	if body != "" {
		_, _ = io.WriteString(w, body)
	}
}
