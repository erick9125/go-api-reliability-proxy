package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRunShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, Options{
			Addr:    "127.0.0.1:0",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down")
	}
}

// Shutdown must drain in-flight requests instead of cancelling them. A request
// context wired to the signal context made the server answer 200 with an empty
// body, which reads as success to every client.
func TestRunDrainsInFlightRequestOnShutdown(t *testing.T) {
	const want = "completed after shutdown started"

	addr := freeAddr(t)
	started := make(chan struct{})
	var handlerCtxErr error

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() {
		runErr <- Run(ctx, Options{
			Addr: addr,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(started)
				select {
				case <-r.Context().Done():
					handlerCtxErr = r.Context().Err()
				case <-time.After(500 * time.Millisecond):
				}
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, want)
			}),
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()
	t.Cleanup(func() { cancel(); <-runErr })

	waitForListener(t, addr)

	type result struct {
		status int
		body   string
		err    error
	}
	resCh := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/slow")
		if err != nil {
			resCh <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		resCh <- result{status: resp.StatusCode, body: string(b), err: err}
	}()

	<-started
	time.Sleep(50 * time.Millisecond)
	cancel() // SIGTERM while the request is still being served

	select {
	case got := <-resCh:
		if got.err != nil {
			t.Fatalf("in-flight request failed during shutdown: %v", got.err)
		}
		if handlerCtxErr != nil {
			t.Errorf("request context was cancelled by shutdown: %v", handlerCtxErr)
		}
		if got.status != http.StatusOK || got.body != want {
			t.Fatalf("in-flight request was not drained: status=%d body=%q, want 200 %q", got.status, got.body, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight request never returned")
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func waitForListener(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server never started listening on %s", addr)
}
