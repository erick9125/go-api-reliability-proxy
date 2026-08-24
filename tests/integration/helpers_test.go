package integration_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erick9125/go-api-reliability-proxy/internal/config"
	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newUpstream(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	return upstream
}

func newProxyServer(t *testing.T, target string, rs []rules.Rule, opts proxy.Options) *httptest.Server {
	t.Helper()
	if opts.Logger == nil {
		opts.Logger = silentLogger()
	}
	h, err := proxy.New(config.Config{
		Version: 1,
		Proxy: config.ProxyConfig{
			Listen: config.DefaultListen,
			Target: target,
		},
		Rules: rs,
	}, opts)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	return server
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
