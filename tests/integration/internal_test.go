package integration_test

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/erick9125/go-api-reliability-proxy/internal/metrics"
	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
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

// Only /__reliability and /__reliability/... are reserved. Paths that merely
// start with those characters belong to the upstream.
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
