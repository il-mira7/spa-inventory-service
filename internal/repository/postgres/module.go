package postgres

import "go.uber.org/fx"

// Module регистрирует слой персистентности PostgreSQL в контейнере зависимостей Uber FX
var Module = fx.Module("postgres",
	fx.Provide(
		NewConnectionPool,
		NewTxManager,
		NewMovementRepository,
		NewStockRepository,
		NewProductRepository,
		NewOrderRepository,
	),
)
