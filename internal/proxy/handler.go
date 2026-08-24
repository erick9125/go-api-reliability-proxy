package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/erick9125/go-api-reliability-proxy/internal/faults"
)

const internalPrefix = "/__reliability"

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, internalPrefix) {
		h.serveInternal(w, r)
		return
	}

	h.metrics.RecordRequest()
	rule := h.matcher.Match(r)
	if rule == nil {
		h.metrics.RecordProxied()
		h.proxy.ServeHTTP(w, r)
		return
	}

	h.metrics.RecordMatch()
	result, err := h.engine.Apply(r.Context(), *rule, w, r)
	if err != nil {
		h.handleEngineError(w, r, err)
		return
	}
	if result.Stop {
		return
	}

	h.metrics.RecordProxied()
	h.proxy.ServeHTTP(w, r)
}

func (h *Handler) handleEngineError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	h.logger.Error("fault engine failed",
		"error", err,
		"method", r.Method,
		"path", r.URL.Path,
	)
	if errors.Is(err, faults.ErrHijackingUnsupported) {
		http.Error(w, "connection reset simulation requires HTTP/1.x", http.StatusInternalServerError)
		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (h *Handler) serveInternal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch r.URL.Path {
	case "/__reliability/health":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case "/__reliability/status":
		snapshot := h.metrics.Snapshot()
		writeJSON(w, http.StatusOK, snapshot)
	default:
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
