package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/il-mira7/spa-inventory-service/internal/config"
	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/handlers"
	appMiddleware "github.com/il-mira7/spa-inventory-service/internal/delivery/http/middleware"
	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/swagger"
	"go.uber.org/fx"
)

// RouterParams содержит все зависимости, необходимые для сборки роутера
type RouterParams struct {
	fx.In

	Config          *config.Config
	Logger          *slog.Logger
	HealthHandler   *handlers.HealthHandler
	MovementHandler *handlers.MovementHandler
	StockHandler    *handlers.StockHandler
	ForecastHandler *handlers.ForecastHandler
	AlertHandler    *handlers.AlertHandler
	RefHandler      *handlers.ReferenceHandler
}

// NewRouter создает и конфигурирует Chi роутер со всеми middleware и маршрутами
func NewRouter(p RouterParams) *chi.Mux {
	r := chi.NewRouter()

	// 1. Стандартные chi middlewares
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(appMiddleware.Recoverer(p.Logger))
	r.Use(appMiddleware.RequestLogger(p.Logger))

	// 2. CORS для взаимодействия с SPA фронтендом
	r.Use(appMiddleware.CORS())

	// 3. Таймаут выполнения запроса
	r.Use(chiMiddleware.Timeout(60 * time.Second))

	// 4. Служебные маршруты и документация
	r.Get("/health", p.HealthHandler.Health)

	// Редирект с GET /docs на /swagger/
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
	r.Get("/docs/*", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})

	swagger.RegisterRoutes(r)

	// 5. Бизнес-маршруты API
	r.Route("/api", func(r chi.Router) {
		r.Route("/movements", func(r chi.Router) {
			r.Post("/", p.MovementHandler.Create)
			r.Get("/", p.MovementHandler.List)
		})

		r.Route("/stock", func(r chi.Router) {
			r.Get("/", p.StockHandler.Summary)
			r.Get("/{sku}", p.StockHandler.Detail)
		})

		r.Post("/forecast", p.ForecastHandler.Calculate)
		r.Get("/alerts", p.AlertHandler.List)

		// Справочники
		r.Get("/locations", p.RefHandler.GetLocations)
		r.Get("/suppliers", p.RefHandler.GetSuppliers)
		r.Get("/products", p.RefHandler.GetProducts)
	})

	return r
}
