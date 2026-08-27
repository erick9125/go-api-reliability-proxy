package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

type Options struct {
	Addr    string
	Handler http.Handler
	Logger  *slog.Logger
}

func Run(ctx context.Context, opts Options) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// ctx only triggers shutdown: parenting request contexts here would cancel
	// in-flight requests on signal, leaving Shutdown nothing to drain.
	srv := &http.Server{
		Addr:              opts.Addr,
		Handler:           opts.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errCh
	case err := <-errCh:
		return err
	}
}
