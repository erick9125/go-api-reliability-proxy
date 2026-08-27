package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type Options struct {
	Addr    string
	Handler http.Handler
	Logger  *slog.Logger
	// OnListen runs once the listener is bound, with the address it actually
	// got. It is the only point where announcing readiness is truthful, and it
	// resolves the real port when Addr asks for :0.
	OnListen func(net.Addr)
}

func Run(ctx context.Context, opts Options) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Bind before serving so a failure here is reported instead of announced as
	// a successful start.
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", opts.Addr)
	if err != nil {
		return err
	}
	if opts.OnListen != nil {
		opts.OnListen(listener.Addr())
	}

	// ctx only triggers shutdown: parenting request contexts here would cancel
	// in-flight requests on signal, leaving Shutdown nothing to drain.
	srv := &http.Server{
		Handler:           opts.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
