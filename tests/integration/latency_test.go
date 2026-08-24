package integration_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

func TestLatencyInjection(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	fixed := rules.Duration{Duration: 20 * time.Millisecond}
	p := newProxyServer(t, upstream.URL, []rules.Rule{{
		Name:  "tiny-delay",
		Match: rules.MatchConfig{Path: "/slow"},
		Effects: rules.Effects{
			Latency: &rules.LatencyConfig{Fixed: &fixed},
		},
	}}, proxy.Options{})

	start := time.Now()
	resp, err := http.Get(p.URL + "/slow")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if elapsed < 15*time.Millisecond {
		t.Fatalf("elapsed %s, expected at least ~20ms of latency", elapsed)
	}
}

func TestTimeoutSimulation(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called")
	}))
	p := newProxyServer(t, upstream.URL, []rules.Rule{{
		Name:  "hang",
		Match: rules.MatchConfig{Path: "/hang"},
		Effects: rules.Effects{
			Timeout: &rules.TimeoutConfig{Duration: rules.Duration{Duration: 20 * time.Millisecond}},
		},
	}}, proxy.Options{})

	resp, err := http.Get(p.URL + "/hang")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
