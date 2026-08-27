package faults

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"

	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

var ErrHijackingUnsupported = errors.New("connection reset simulation requires HTTP/1.x hijacking")

// applyReset reports whether the request was handled and whether a reset
// actually reached the client; hijacking failures make the two differ.
func (e *Engine) applyReset(ctx context.Context, rule rules.Rule, w http.ResponseWriter, r *http.Request) (stop, faulted bool, err error) {
	probability := 1.0
	if rule.Effects.Reset.Probability != nil {
		probability = *rule.Effects.Reset.Probability
	}
	if !e.shouldTrigger(probability) {
		return false, false, nil
	}
	// A client that already left cannot observe a reset.
	if err := ctx.Err(); err != nil {
		return true, false, err
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return true, false, ErrHijackingUnsupported
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		return true, false, err
	}
	// The connection is ours now, so Close failures are logged, not returned:
	// the error handler would write onto a hijacked ResponseWriter.
	forceReset(conn)
	if err := conn.Close(); err != nil {
		e.logger.Warn("closing hijacked connection",
			"error", err,
			"rule", rule.Name,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}
	e.logFault(rule, r, "reset")
	e.metrics.RecordFault()
	return true, true, nil
}

// forceReset makes the following Close emit a TCP RST instead of a graceful
// FIN, which the client would see as EOF rather than a connection reset.
func forceReset(conn net.Conn) {
	for {
		switch c := conn.(type) {
		case *net.TCPConn:
			_ = c.SetLinger(0)
			return
		case *tls.Conn:
			conn = c.NetConn()
		default:
			return
		}
	}
}
