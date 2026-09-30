package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/il-mira7/spa-inventory-service/internal/config"
	"github.com/il-mira7/spa-inventory-service/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/fx"
)

// NewConnectionPool инициализирует пул подключений к PostgreSQL и запускает миграции базы данных.
func NewConnectionPool(lc fx.Lifecycle, cfg *config.Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	ctx := context.Background()
	poolCfg, err := buildPoolConfig(cfg)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx connection pool: %w", err)
	}

	if err := verifyAndMigrate(ctx, pool, cfg, logger); err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("closing PostgreSQL connection pool")
			pool.Close()
			return nil
		},
	})

	return pool, nil
}

func buildPoolConfig(cfg *config.Config) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DATABASE_URL: %w", err)
	}

	poolCfg.MaxConns = cfg.DBPoolMaxConns
	poolCfg.MinConns = cfg.DBPoolMinConns
	poolCfg.MaxConnLifetime = cfg.DBPoolMaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.DBPoolMaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.DBPoolHealthCheckPeriod

	return poolCfg, nil
}

func verifyAndMigrate(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, logger *slog.Logger) error {
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed on startup: %w", err)
	}

	logger.Info("connected to PostgreSQL successfully",
		slog.Int("max_conns", int(cfg.DBPoolMaxConns)),
		slog.Int("min_conns", int(cfg.DBPoolMinConns)),
	)

	return runMigrations(pool, logger)
}

func runMigrations(pool *pgxpool.Pool, logger *slog.Logger) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func(db *sql.DB) {
		_ = db.Close()
	}(sqlDB)

	if err := migrations.Up(sqlDB); err != nil {
		return fmt.Errorf("failed to apply database migrations: %w", err)
	}

	logger.Info("database migrations applied successfully")
	return nil
}
