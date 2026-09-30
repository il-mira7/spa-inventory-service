package dto

import "github.com/il-mira7/spa-inventory-service/internal/domain"

// AlertResponse DTO элемента предупреждения GET /api/alerts
type AlertResponse struct {
	Level    domain.AlertLevel      `json:"level"`
	Type     domain.AlertType       `json:"type"`
	SKU      string                 `json:"sku"`
	Name     string                 `json:"name"`
	Location string                 `json:"location"`
	Message  string                 `json:"message"`
	Metrics  map[string]interface{} `json:"metrics"`
}
