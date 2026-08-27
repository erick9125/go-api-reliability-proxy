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

// Readiness must be announced after the bind, never before: a failed startup
// used to be logged as a successful one.
func TestOnListenReportsTheBoundAddress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addrCh := make(chan net.Addr, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, Options{
			Addr:     "127.0.0.1:0",
			Handler:  http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
			OnListen: func(addr net.Addr) { addrCh <- addr },
		})
	}()

	select {
	case addr := <-addrCh:
		// :0 must be resolved to the port the OS actually handed out.
		_, port, err := net.SplitHostPort(addr.String())
		if err != nil {
			t.Fatalf("address %q is not host:port: %v", addr, err)
		}
		if port == "0" || port == "" {
			t.Fatalf("address %q still reports the placeholder port", addr)
		}
		// The listener must already accept connections by the time this fires.
		conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
		if err != nil {
			t.Fatalf("listener not accepting at OnListen time: %v", err)
		}
		conn.Close()
	case err := <-errCh:
		t.Fatalf("Run returned before listening: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("OnListen was never called")
	}

	cancel()
	<-errCh
}

func TestRunFailsWithoutAnnouncingWhenPortIsTaken(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()

	announced := false
	err = Run(context.Background(), Options{
		Addr:     busy.Addr().String(),
		Handler:  http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		OnListen: func(net.Addr) { announced = true },
	})

	if err == nil {
		t.Fatal("expected Run to fail on an address already in use")
	}
	if announced {
		t.Fatal("startup was announced even though the bind failed")
	}
}

// Shutdown must drain in-flight requests, not cancel them into an empty 200.
func TestRunDrainsInFlightRequestOnShutdown(t *testing.T) {
	const want = "completed after shutdown started"

	started := make(chan struct{})
	var handlerCtxErr error

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	addrCh := make(chan net.Addr, 1)
	go func() {
		runErr <- Run(ctx, Options{
			Addr: "127.0.0.1:0",
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
			Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
			OnListen: func(a net.Addr) { addrCh <- a },
		})
	}()
	t.Cleanup(func() { cancel(); <-runErr })

	var addr string
	select {
	case a := <-addrCh:
		addr = a.String()
	case <-time.After(3 * time.Second):
		t.Fatal("server never started listening")
	}

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
