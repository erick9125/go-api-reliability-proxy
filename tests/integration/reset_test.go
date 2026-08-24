package integration_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

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
}
