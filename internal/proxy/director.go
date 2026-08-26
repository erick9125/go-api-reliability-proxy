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

// Upstream transport limits. http.DefaultTransport has no response header
// timeout, so a hung upstream would pin a connection and its goroutine forever
// — likely for a tool whose whole job is provoking adverse conditions. It also
// caps idle connections per host at 2, which throttles a proxy under load.
//
// Injected latency runs before the request is forwarded, so it never counts
// against responseHeaderTimeout; only genuine upstream slowness does.
const (
	dialTimeout           = 10 * time.Second
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
