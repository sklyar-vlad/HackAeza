package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/config"
)

func New(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error) {
	settings, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL configuration")
	}
	settings.MaxConns = cfg.MaxConnections
	settings.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	settings.ConnConfig.RuntimeParams["timezone"] = "UTC"
	ctx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, settings)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect PostgreSQL: %w", err)
	}
	return pool, nil
}
