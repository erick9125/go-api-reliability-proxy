package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// Upstream limits. http.DefaultTransport sets no response header timeout and
// allows only 2 idle connections per host, which throttles a proxy.
const (
	dialTimeout = 10 * time.Second
	// Injected latency runs before forwarding, so it never counts against this.
	responseHeaderTimeout = 30 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	idleConnTimeout       = 90 * time.Second
	maxIdleConnsPerHost   = 100
	expectContinueTimeout = 1 * time.Second
)

func newTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}
}

func newReverseProxy(target *url.URL, logger *slog.Logger) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Transport: newTransport(),
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			logger.Error("upstream request failed",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
			)
			w.WriteHeader(http.StatusBadGateway)
		},
		FlushInterval: 100 * time.Millisecond,
	}
}
