package integration_test

import (
	"errors"
	"io"
	"net/http"
	"syscall"
	"testing"

	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

// wsaeconnreset is WSAECONNRESET, the Windows counterpart of ECONNRESET.
const wsaeconnreset = syscall.Errno(10054)

func isConnectionReset(err error) bool {
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == wsaeconnreset
}

func TestConnectionReset(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called")
	}))
	p := newProxyServer(t, upstream.URL, []rules.Rule{{
		Name:  "reset",
		Match: rules.MatchConfig{Path: "/reset"},
		Effects: rules.Effects{
			Reset: &rules.ResetConfig{},
		},
	}}, proxy.Options{})

	resp, err := http.Get(p.URL + "/reset")
	if err == nil {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		t.Fatal("expected connection error from reset simulation")
	}

	// A graceful close surfaces as EOF, which is a different fault than the one
	// this effect claims to inject. Assert the real thing.
	if !isConnectionReset(err) {
		t.Fatalf("expected a connection reset, got %v", err)
	}
}
