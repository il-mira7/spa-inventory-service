package domain

// AlertLevel уровень важности предупреждения
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// AlertType тип предупреждения по складу
type AlertType string

const (
	AlertTypeStockoutRisk AlertType = "stockout_risk" // Риск дефицита (запас в днях < срок поставки)
	AlertTypeExpiryRisk   AlertType = "expiry_risk"   // Приближение срока годности партии
	AlertTypeDeadStock    AlertType = "dead_stock"    // Отсутствие движения по позиции (неликвид)
)

// Alert представляет предупреждение по состоянию склада для GET /api/alerts
type Alert struct {
	Level      AlertLevel             `json:"level"`
	Type       AlertType              `json:"type"`
	SKU        string                 `json:"sku"`
	Name       string                 `json:"name"`
	LocationID string                 `json:"location"`
	Message    string                 `json:"message"`
	Metrics    map[string]interface{} `json:"metrics"`
}
