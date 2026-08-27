package integration_test

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/erick9125/go-api-reliability-proxy/internal/faults"
	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

func TestSyntheticFailureDoesNotHitUpstream(t *testing.T) {
	var hits atomic.Int64
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	p := newProxyServer(t, upstream.URL, []rules.Rule{{
		Name:  "users-down",
		Match: rules.MatchConfig{Path: "/users/*"},
		Effects: rules.Effects{
			Response: &rules.ResponseConfig{Status: http.StatusServiceUnavailable},
		},
	}}, proxy.Options{})

	resp, err := http.Get(p.URL + "/users/123")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits=%d, want 0", hits.Load())
	}
}

func TestFailureProbabilityWithFixedRandom(t *testing.T) {
	var hits atomic.Int64
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	prob := 0.25
	p := newProxyServer(t, upstream.URL, []rules.Rule{{
		Name:  "flaky",
		Match: rules.MatchConfig{Path: "/inventory/*"},
		Effects: rules.Effects{
			Failure: &rules.FailureConfig{
				Probability: &prob,
				Status:      http.StatusServiceUnavailable,
			},
		},
	}}, proxy.Options{Faults: faults.Options{Random: faults.FixedRandom{Value: 0.10}}})

	resp, err := http.Get(p.URL + "/inventory/1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits=%d", hits.Load())
	}
}

// Several values must arrive as several headers, not one joined string.
func TestSyntheticResponseSendsRepeatedHeaders(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called")
	}))
	p := newProxyServer(t, upstream.URL, []rules.Rule{{
		Name:  "unauthorized",
		Match: rules.MatchConfig{Path: "/private"},
		Effects: rules.Effects{
			Response: &rules.ResponseConfig{
				Status: http.StatusUnauthorized,
				Headers: map[string]rules.HeaderValues{
					"Set-Cookie": {"session=; Max-Age=0", "csrf=; Max-Age=0"},
				},
			},
		},
	}}, proxy.Options{})

	resp, err := http.Get(p.URL + "/private")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	got := resp.Header.Values("Set-Cookie")
	if len(got) != 2 {
		t.Fatalf("Set-Cookie = %#v, want 2 separate values", got)
	}
	if got[0] != "session=; Max-Age=0" || got[1] != "csrf=; Max-Age=0" {
		t.Fatalf("Set-Cookie = %#v", got)
	}
}

func TestRateLimitSimulation(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called")
	}))
	p := newProxyServer(t, upstream.URL, []rules.Rule{{
		Name: "rate-limit-users",
		Match: rules.MatchConfig{
			Path:    "/users/*",
			Methods: []string{http.MethodGet},
		},
		Effects: rules.Effects{
			Response: &rules.ResponseConfig{
				Status:  http.StatusTooManyRequests,
				Headers: map[string]rules.HeaderValues{"Retry-After": {"10"}},
				Body:    `{"error":"rate limited"}`,
			},
		},
	}}, proxy.Options{})

	resp, err := http.Get(p.URL + "/users/1")
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") != "10" {
		t.Fatalf("Retry-After=%q", resp.Header.Get("Retry-After"))
	}
	if body != `{"error":"rate limited"}` {
		t.Fatalf("body=%q", body)
	}
}
