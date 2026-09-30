package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/dto"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBPinger описывает интерфейс проверки доступности базы данных
type DBPinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler обслуживает эндпоинты проверки состояния приложения
type HealthHandler struct {
	pinger DBPinger
}

// NewHealthHandler создает обработчик с пулом подключений pgxpool
func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{pinger: pool}
}

// NewHealthHandlerWithPinger создает обработчик с кастомным пингером (для тестов)
func NewHealthHandlerWithPinger(pinger DBPinger) *HealthHandler {
	return &HealthHandler{pinger: pinger}
}

// Health обрабатывает GET /health
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC().Format(time.RFC3339)

	if h.pinger != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := h.pinger.Ping(ctx); err != nil {
			JSON(w, http.StatusServiceUnavailable, dto.HealthResponse{
				Status:    "degraded",
				Timestamp: now,
				Database:  "disconnected",
			})
			return
		}
	}

	JSON(w, http.StatusOK, dto.HealthResponse{
		Status:    "ok",
		Timestamp: now,
		Database:  "connected",
		Version:   "1.0.0",
	})
}
