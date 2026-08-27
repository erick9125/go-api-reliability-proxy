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

// isInternalPath reserves /__reliability and /__reliability/*, but not every
// path that merely starts with those characters.
func isInternalPath(path string) bool {
	return path == internalPrefix || strings.HasPrefix(path, internalPrefix+"/")
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isInternalPath(r.URL.Path) {
		h.serveInternal(w, r)
		return
	}

	h.metrics.RecordRequest()
	rule, matched := h.matcher.Match(r)
	if !matched {
		h.metrics.RecordProxied()
		h.proxy.ServeHTTP(w, r)
		return
	}

	h.metrics.RecordMatch()
	result, err := h.engine.Apply(r.Context(), rule, w, r)
	if result.Faulted {
		h.metrics.RecordRequestFaulted()
	}
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
	// HEAD is served by net/http from the GET handler, so both are allowed.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
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
