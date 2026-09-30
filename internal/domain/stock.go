package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// StockItem представляет сводный остаток позиции на складе для эндпоинта GET /api/stock
type StockItem struct {
	SKU                 string           `json:"sku"`
	Name                string           `json:"name"`
	LocationID          string           `json:"location"`
	Unit                string           `json:"unit"`
	CurrentStock        decimal.Decimal  `json:"current_stock"`
	AvgDailyConsumption decimal.Decimal  `json:"avg_daily_consumption"`
	StockDays           *decimal.Decimal `json:"stock_days"`
	NearestExpiryDate   *time.Time       `json:"nearest_expiry_date,omitempty"`
}

// LocationStockBreakdown детализирует остатки позиции по объекту и активным партиям
type LocationStockBreakdown struct {
	LocationID   string          `json:"location_id"`
	LocationName string          `json:"location_name"`
	TotalStock   decimal.Decimal `json:"total_stock"`
	Batches      []BatchStock    `json:"batches"`
}

// StockDetail представляет полную детализацию по позиции для эндпоинта GET /api/stock/{sku}
type StockDetail struct {
	SKU       string                   `json:"sku"`
	Name      string                   `json:"name"`
	Unit      string                   `json:"unit"`
	Locations []LocationStockBreakdown `json:"locations"`
}
