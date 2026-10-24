// Package httpserver manages the HTTP server lifecycle, independently of business handlers.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Run follows Flagman's signal -> bounded Shutdown -> forced Close lifecycle.
// The caller owns signal handling and closes shared resources after Run returns.
func Run(ctx context.Context, server *http.Server, logger *slog.Logger, shutdownTimeout time.Duration) error {
	if ctx.Err() != nil {
		return nil
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}

	result := make(chan error, 1)
	go func() {
		result <- server.Serve(listener)
	}()
	logger.Info("service started", "addr", listener.Addr().String())

	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutdown signal received")
	// Keep request contexts alive while draining; the signal context is already cancelled.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		closeErr := server.Close()
		<-result
		return errors.Join(fmt.Errorf("shutdown HTTP: %w", err), closeErr)
	}

	if err := <-result; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	logger.Info("server stopped gracefully")
	return nil
}
