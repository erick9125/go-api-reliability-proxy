package integration_test

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
)

func TestPassthroughStatusBodyAndHeaders(t *testing.T) {
	var hits atomic.Int64
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/hello" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	p := newProxyServer(t, upstream.URL, nil, proxy.Options{})

	resp, err := http.Get(p.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if body != "hello" {
		t.Fatalf("body=%q", body)
	}
	if resp.Header.Get("X-Upstream") != "yes" {
		t.Fatalf("missing upstream header")
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits=%d", hits.Load())
	}
}

func TestPassthroughRequestBody(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}))
	p := newProxyServer(t, upstream.URL, nil, proxy.Options{})

	resp, err := http.Post(p.URL+"/echo", "text/plain", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if got := readBody(t, resp); got != "payload" {
		t.Fatalf("body=%q", got)
	}
}

func TestForwardedHeaders(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-For") == "" {
			t.Error("expected X-Forwarded-For")
		}
		if r.Header.Get("X-Forwarded-Host") == "" {
			t.Error("expected X-Forwarded-Host")
		}
		if r.Header.Get("X-Forwarded-Proto") == "" {
			t.Error("expected X-Forwarded-Proto")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	p := newProxyServer(t, upstream.URL, nil, proxy.Options{})
	resp, err := http.Get(p.URL + "/fwd")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestUpstreamDownReturns502(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	p := newProxyServer(t, upstream.URL, nil, proxy.Options{})
	upstream.Close()

	resp, err := http.Get(p.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
