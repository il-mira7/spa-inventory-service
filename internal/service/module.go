package service

import "go.uber.org/fx"

// Module регистрирует сервисы предметной области в DI контейнере Uber FX
var Module = fx.Module("service",
	fx.Provide(
		NewMovementService,
		NewStockService,
		NewForecastService,
		NewAlertService,
	),
)
