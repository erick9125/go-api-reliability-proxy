package faults

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"

	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

var ErrHijackingUnsupported = errors.New("connection reset simulation requires HTTP/1.x hijacking")

func (e *Engine) applyReset(rule rules.Rule, w http.ResponseWriter, r *http.Request) (bool, error) {
	probability := 1.0
	if rule.Effects.Reset.Probability != nil {
		probability = *rule.Effects.Reset.Probability
	}
	if !e.shouldFail(probability) {
		return false, nil
	}
	e.logFault(rule, r, "reset")
	e.recordFault()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return true, ErrHijackingUnsupported
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		return true, err
	}
	forceReset(conn)
	return true, conn.Close()
}

// forceReset makes the following Close send a TCP RST instead of a graceful
// FIN. Without it the client observes EOF, which is a different failure than
// the connection reset this effect is named after: net/http silently retries
// an idempotent request that dies with EOF on a reused connection.
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
