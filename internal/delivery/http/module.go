package http

import (
	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/handlers"
	"go.uber.org/fx"
)

// Module предоставляет все зависимости транспортного HTTP слоя для Uber FX
var Module = fx.Module("delivery_http",
	fx.Provide(
		handlers.NewHealthHandler,
		handlers.NewMovementHandler,
		handlers.NewStockHandler,
		handlers.NewForecastHandler,
		handlers.NewAlertHandler,
		handlers.NewReferenceHandler,
		NewRouter,
		NewServer,
	),
	fx.Invoke(func(s *Server) {}),
)
