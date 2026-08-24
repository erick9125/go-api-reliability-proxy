package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

func TestClientCancellationDuringLatency(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called after cancellation")
	}))
	fixed := rules.Duration{Duration: time.Second}
	p := newProxyServer(t, upstream.URL, []rules.Rule{{
		Name:  "cancel-me",
		Match: rules.MatchConfig{Path: "/cancel"},
		Effects: rules.Effects{
			Latency: &rules.LatencyConfig{Fixed: &fixed},
		},
	}}, proxy.Options{})

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL+"/cancel", nil)
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		errCh <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("request did not return after cancellation")
	}
}
