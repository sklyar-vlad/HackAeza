package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/sklyar-vlad/HackAeza/backend/internal/analytics"
	"github.com/sklyar-vlad/HackAeza/backend/internal/auth"
	"github.com/sklyar-vlad/HackAeza/backend/internal/config"
	"github.com/sklyar-vlad/HackAeza/backend/internal/customers"
	"github.com/sklyar-vlad/HackAeza/backend/internal/handler"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"github.com/sklyar-vlad/HackAeza/backend/internal/platform/migrations"
	"github.com/sklyar-vlad/HackAeza/backend/internal/platform/postgres"
	"github.com/sklyar-vlad/HackAeza/backend/internal/source"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	if err := run(logger); err != nil {
		logger.Error("server stopped", zap.Error(err))
		_ = logger.Sync()
		os.Exit(1)
	}
}

func run(logger *zap.Logger) error {
	cfg := config.NewConfig()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.New(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if len(os.Args) != 3 {
			return errors.New("usage: /server migrate up|down|status|version")
		}
		migrationCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		return migrations.Run(migrationCtx, pool, os.Args[2], os.Stdout)
	}
	startupCtx, cancelStartup := context.WithTimeout(ctx, 15*time.Second)
	defer cancelStartup()
	if err := migrations.Check(startupCtx, pool); err != nil {
		return fmt.Errorf("check database migrations: %w", err)
	}
	users := auth.NewPostgres(pool)
	if len(os.Args) > 1 {
		return manageUser(ctx, users, os.Args[1:])
	}
	lifetime, err := config.SessionLifetime()
	if err != nil {
		return err
	}
	service, err := auth.New(users, users, lifetime)
	if err != nil {
		return err
	}
	authHandler := auth.NewHandler(service, slog.Default())

	gin.SetMode(gin.ReleaseMode)
	router := handler.NewRouter(logger, pool)
	api := router.Group("/api/v1")
	queue := jobs.NewRepository(pool)
	source.NewHandler(source.NewService(source.NewRepository(pool, queue)), queue.SessionActor).Register(api)
	customers.NewHandler(customers.NewService(customers.NewRepository(pool))).Register(api)
	analytics.NewHandler(analytics.NewService(analytics.NewRepository(pool))).Register(api)

	router.GET("/readyz", func(c *gin.Context) {
		probeCtx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		var loaded bool
		err := pool.QueryRow(probeCtx, `SELECT EXISTS (SELECT 1 FROM published_state p
   JOIN calculation_versions c ON c.version = p.active_calculation_version WHERE c.source_version IS NOT NULL)`).Scan(&loaded)
		if err != nil {
			auth.JSON(c.Writer, c.Request, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "database": "down", "snapshotLoaded": false})
			return
		}
		auth.JSON(c.Writer, c.Request, http.StatusOK, map[string]any{"status": "ready", "database": "up", "snapshotLoaded": loaded})
	})
	public, protected := http.NewServeMux(), http.NewServeMux()
	authHandler.Register(public, protected)
	protected.Handle("/", router)
	public.Handle("/api/v1/", authHandler.RequireAuth(protected))
	public.Handle("/", router)
	cleanupCtx, cancelCleanup := context.WithCancel(ctx)
	defer cancelCleanup()
	go pruneSessions(cleanupCtx, users, logger)

	server := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Server.Port),
		Handler:           auth.RequestID(public),
		ErrorLog:          zap.NewStdLog(logger),
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	return serve(ctx, server, logger, cfg.Server.ShutdownTimeout)
}

func serve(ctx context.Context, server *http.Server, logger *zap.Logger, shutdownTimeout time.Duration) error {
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
	logger.Info("service started", zap.String("addr", listener.Addr().String()))

	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
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

func pruneSessions(ctx context.Context, users *auth.Postgres, logger *zap.Logger) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := users.Prune(cleanupCtx); err != nil && ctx.Err() == nil {
				logger.Error("prune sessions", zap.Error(err))
			}
			cancel()
		}
	}
}
