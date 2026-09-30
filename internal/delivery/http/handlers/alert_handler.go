package handlers

import (
	"net/http"

	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/dto"
	"github.com/il-mira7/spa-inventory-service/internal/service"
)

// AlertHandler обрабатывает запросы к реестру предупреждений
type AlertHandler struct {
	alertService service.AlertService
}

// NewAlertHandler создает новый экземпляр обработчика алертов
func NewAlertHandler(alertService service.AlertService) *AlertHandler {
	return &AlertHandler{alertService: alertService}
}

// List обрабатывает GET /api/alerts?location=
func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	var locParam *string
	loc := r.URL.Query().Get("location")
	if loc == "" {
		loc = r.URL.Query().Get("location_id")
	}
	if loc != "" {
		locParam = &loc
	}

	alerts, err := h.alertService.GetAlerts(r.Context(), locParam)
	if err != nil {
		Error(w, err)
		return
	}

	resp := make([]dto.AlertResponse, 0, len(alerts))
	for _, a := range alerts {
		resp = append(resp, dto.AlertResponse{
			Level:    a.Level,
			Type:     a.Type,
			SKU:      a.SKU,
			Name:     a.Name,
			Location: a.LocationID,
			Message:  a.Message,
			Metrics:  a.Metrics,
		})
	}

	JSON(w, http.StatusOK, resp)
}
