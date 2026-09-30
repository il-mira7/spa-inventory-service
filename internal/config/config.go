package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config хранит параметры конфигурации приложения.
type Config struct {
	AppEnv      string        `env:"APP_ENV" envDefault:"development"`
	HTTPPort    int           `env:"HTTP_PORT" envDefault:"8080"`
	HTTPTimeout time.Duration `env:"HTTP_TIMEOUT" envDefault:"15s"`
	DatabaseURL string        `env:"DATABASE_URL" envDefault:"postgres://postgres:postgres@localhost:5432/spa_inventory?sslmode=disable"`
	LogLevel    string        `env:"LOG_LEVEL" envDefault:"info"`

	// Параметры пула подключений к БД (pgxpool)
	DBPoolMaxConns          int32         `env:"DB_POOL_MAX_CONNS" envDefault:"25"`
	DBPoolMinConns          int32         `env:"DB_POOL_MIN_CONNS" envDefault:"5"`
	DBPoolMaxConnLifetime   time.Duration `env:"DB_POOL_MAX_CONN_LIFETIME" envDefault:"1h"`
	DBPoolMaxConnIdleTime   time.Duration `env:"DB_POOL_MAX_CONN_IDLE_TIME" envDefault:"30m"`
	DBPoolHealthCheckPeriod time.Duration `env:"DB_POOL_HEALTH_CHECK_PERIOD" envDefault:"1m"`
}

// NewConfig считывает конфигурацию из .env (если есть) и системных переменных окружения.
func NewConfig() (*Config, error) {
	// Игнорируем ошибку, если файла .env нет (актуально для production/docker)
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	return cfg, nil
}
