package app

import (
	"context"
	"log/slog"

	"github.com/il-mira7/spa-inventory-service/internal/config"
	deliveryHTTP "github.com/il-mira7/spa-inventory-service/internal/delivery/http"
	"github.com/il-mira7/spa-inventory-service/internal/repository/postgres"
	"github.com/il-mira7/spa-inventory-service/internal/service"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// Options возвращает полный список опций и модулей приложения Uber FX
func Options() []fx.Option {
	return []fx.Option{
		// 1. Конфигурация и структурированное логирование
		fx.Provide(
			config.NewConfig,
			NewLogger,
		),
		fx.WithLogger(func(logger *slog.Logger) fxevent.Logger {
			return NewFxLogger(logger)
		}),

		// 2. Слой данных (PostgreSQL + автомиграции Goose + TxManager)
		postgres.Module,

		// 3. Сервисный слой бизнес-логики
		service.Module,

		// 4. Транспортный HTTP-слой (Chi + Handlers + Swagger UI)
		deliveryHTTP.Module,

		// 5. Хуки жизненного цикла приложения
		fx.Invoke(registerAppLifecycle),
	}
}

// NewApp собирает и конфигурирует полное приложение Uber FX
func NewApp() *fx.App {
	return fx.New(Options()...)
}

func registerAppLifecycle(lc fx.Lifecycle, logger *slog.Logger, cfg *config.Config) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("ABS Inventory & Procurement Service starting",
				slog.String("env", cfg.AppEnv),
				slog.Int("port", cfg.HTTPPort),
				slog.String("log_level", cfg.LogLevel),
			)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("ABS Inventory & Procurement Service stopped")
			return nil
		},
	})
}
