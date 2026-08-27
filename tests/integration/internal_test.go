package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erick9125/go-api-reliability-proxy/internal/config"
	"github.com/erick9125/go-api-reliability-proxy/internal/metrics"
	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

func TestInternalHealthAndStatus(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	p := newProxyServer(t, upstream.URL, nil, proxy.Options{})

	health, err := http.Get(p.URL + "/__reliability/health")
	if err != nil {
		t.Fatal(err)
	}
	if health.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d", health.StatusCode)
	}
	var healthBody map[string]string
	if err := json.NewDecoder(health.Body).Decode(&healthBody); err != nil {
		t.Fatal(err)
	}
	health.Body.Close()
	if healthBody["status"] != "ok" {
		t.Fatalf("health body=%v", healthBody)
	}

	resp, err := http.Get(p.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	status, err := http.Get(p.URL + "/__reliability/status")
	if err != nil {
		t.Fatal(err)
	}
	var snap metrics.Snapshot
	if err := json.NewDecoder(status.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	status.Body.Close()
	if snap.Requests < 1 || snap.Proxied < 1 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

// Paths that merely start with the reserved prefix belong to the upstream.
func TestPathsAdjacentToInternalNamespaceAreProxied(t *testing.T) {
	paths := []string{"/__reliabilityX/report", "/__reliability-report", "/__reliability_v2"}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			var reached atomic.Bool
			upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != path {
					t.Errorf("upstream saw %q, want %q", r.URL.Path, path)
				}
				reached.Store(true)
				w.WriteHeader(http.StatusOK)
			}))
			p := newProxyServer(t, upstream.URL, nil, proxy.Options{})

			resp, err := http.Get(p.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if !reached.Load() {
				t.Fatalf("%s was intercepted (status %d) instead of proxied", path, resp.StatusCode)
			}
		})
	}
}

func TestInternalEndpointsRejectOtherMethodsWithAllow(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called")
	}))
	p := newProxyServer(t, upstream.URL, nil, proxy.Options{})

	req, err := http.NewRequest(http.MethodPost, p.URL+"/__reliability/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
	if got := resp.Header.Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want %q", got, "GET, HEAD")
	}
}

// A latency the client abandoned must not be counted as an injected fault.
func TestAbandonedLatencyIsNotCounted(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	fixed := rules.Duration{Duration: 2 * time.Second}
	h, err := proxy.New(config.Config{
		Version: 1,
		Proxy:   config.ProxyConfig{Listen: config.DefaultListen, Target: upstream.URL},
		Rules: []rules.Rule{{
			Name:    "slow",
			Match:   rules.MatchConfig{Path: "/slow"},
			Effects: rules.Effects{Latency: &rules.LatencyConfig{Fixed: &fixed}},
		}},
	}, proxy.Options{Logger: silentLogger()})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	client := &http.Client{Timeout: 100 * time.Millisecond}
	resp, err := client.Get(srv.URL + "/slow")
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected the client to time out")
	}
	time.Sleep(50 * time.Millisecond)

	snap := h.Metrics().Snapshot()
	if snap.FaultsInjected != 0 {
		t.Errorf("faultsInjected = %d, want 0 for a latency the client abandoned", snap.FaultsInjected)
	}
	if snap.RequestsFaulted != 0 {
		t.Errorf("requestsFaulted = %d, want 0", snap.RequestsFaulted)
	}
	if snap.Matched != 1 {
		t.Errorf("matched = %d, want 1", snap.Matched)
	}
}

// A rule with two effects counts two faults but only one faulted request.
func TestCombinedEffectsCountOncePerRequest(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	fixed := rules.Duration{Duration: time.Millisecond}
	probability := 1.0
	h, err := proxy.New(config.Config{
		Version: 1,
		Proxy:   config.ProxyConfig{Listen: config.DefaultListen, Target: upstream.URL},
		Rules: []rules.Rule{{
			Name:  "slow-and-broken",
			Match: rules.MatchConfig{Path: "/both"},
			Effects: rules.Effects{
				Latency: &rules.LatencyConfig{Fixed: &fixed},
				Failure: &rules.FailureConfig{Probability: &probability, Status: http.StatusServiceUnavailable},
			},
		}},
	}, proxy.Options{Logger: silentLogger()})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/both")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	snap := h.Metrics().Snapshot()
	if snap.FaultsInjected != 2 {
		t.Errorf("faultsInjected = %d, want 2 (latency and failure)", snap.FaultsInjected)
	}
	if snap.RequestsFaulted != 1 {
		t.Errorf("requestsFaulted = %d, want 1", snap.RequestsFaulted)
	}
	if snap.Proxied != 0 {
		t.Errorf("proxied = %d, want 0", snap.Proxied)
	}
}

func TestInternalNamespaceNotProxied(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("internal path should not be proxied: %s", r.URL.Path)
	}))
	p := newProxyServer(t, upstream.URL, nil, proxy.Options{})
	resp, err := http.Get(p.URL + "/__reliability/unknown")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
