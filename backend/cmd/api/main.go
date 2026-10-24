package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/auth"
	"github.com/sklyar-vlad/HackAeza/backend/internal/config"
	"github.com/sklyar-vlad/HackAeza/backend/internal/platform/httpserver"
	"github.com/sklyar-vlad/HackAeza/backend/internal/platform/migrations"
)

func main() {
	if err := run(); err != nil {
		message := "server stopped"
		if len(os.Args) > 1 {
			message = "command failed"
		}
		slog.Error(message, "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.NewConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid DATABASE_URL") // Parser errors may include credentials.
	}
	poolConfig.MaxConns = 10
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return errors.New("cannot configure PostgreSQL pool")
	}
	defer pool.Close()
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if len(os.Args) != 3 {
			return errors.New("usage: /server migrate up|down|status|version")
		}
		migrationCtx, cancelMigration := context.WithTimeout(ctx, 5*time.Minute)
		defer cancelMigration()
		return migrations.Run(migrationCtx, pool, os.Args[2], os.Stdout)
	}
	startupCtx, cancelStartup := context.WithTimeout(ctx, 15*time.Second)
	defer cancelStartup()
	users := auth.NewPostgres(pool)
	if err := migrations.Check(startupCtx, pool); err != nil {
		return errors.New("database schema is not ready: run task migrate:up and check PostgreSQL")
	}
	if len(os.Args) > 1 {
		return manageUser(ctx, users, os.Args[1:])
	}
	service, err := auth.New(users, users, cfg.SessionLifetime)
	if err != nil {
		return err
	}
	handler := auth.NewHandler(service, slog.Default())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("{\"status\":\"ok\"}\n")); err != nil {
			slog.Error("write health response", "error", err)
		}
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		probeCtx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := pool.Ping(probeCtx); err != nil {
			auth.JSON(w, r, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "database": "down", "snapshotLoaded": false})
			return
		}
		auth.JSON(w, r, http.StatusOK, map[string]any{"status": "ready", "database": "up", "snapshotLoaded": false})
	})
	api := http.NewServeMux()
	handler.Register(mux, api)
	// Register future business handlers on api: every route inherits authentication.
	mux.Handle("/api/v1/", handler.RequireAuth(api))
	cleanupCtx, cancelCleanup := context.WithCancel(ctx)
	defer cancelCleanup()
	go pruneSessions(cleanupCtx, users)

	server := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Server.Port),
		Handler:           auth.RequestID(mux),
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	return httpserver.Run(ctx, server, slog.Default(), cfg.Server.ShutdownTimeout)
}

func pruneSessions(ctx context.Context, users *auth.Postgres) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pruneCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := users.Prune(pruneCtx)
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.Error("prune auth sessions failed", "error", err)
			}
		}
	}
}
