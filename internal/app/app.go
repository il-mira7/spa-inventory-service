package app

import (
	"context"
	"log/slog"

	"github.com/il-mira7/spa-inventory-service/internal/config"
	"go.uber.org/fx"
)

// BaseModule объединяет базовые инфраструктурные провайдеры (Config, Logger).
var BaseModule = fx.Module("base",
	fx.Provide(
		config.NewConfig,
		NewLogger,
	),
	fx.WithLogger(NewFxLogger),
	fx.Invoke(registerLifecycleHooks),
)

// registerLifecycleHooks регистрирует стартовые хуки приложения для верификации работы.
func registerLifecycleHooks(lc fx.Lifecycle, logger *slog.Logger, cfg *config.Config) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("application bootstrap initialized",
				slog.String("env", cfg.AppEnv),
				slog.Int("port", cfg.HTTPPort),
				slog.String("log_level", cfg.LogLevel),
			)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("application shutting down...")
			return nil
		},
	})
}
