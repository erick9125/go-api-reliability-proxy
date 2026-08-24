package server

import (
	"context"
	"io"
	"log/slog"
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
