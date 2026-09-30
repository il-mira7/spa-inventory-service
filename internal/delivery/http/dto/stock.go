package dto

import (
	"github.com/shopspring/decimal"
)

// StockItemResponse DTO элемента сводки остатков GET /api/stock
type StockItemResponse struct {
	SKU                 string           `json:"sku"`
	Name                string           `json:"name"`
	Location            string           `json:"location"`
	Unit                string           `json:"unit"`
	CurrentStock        decimal.Decimal  `json:"current_stock"`
	AvgDailyConsumption decimal.Decimal  `json:"avg_daily_consumption"`
	StockDays           *decimal.Decimal `json:"stock_days"`
	NearestExpiryDate   *string          `json:"nearest_expiry_date,omitempty"`
}

// StockDetailResponse DTO подробной информации по товару GET /api/stock/{sku}
type StockDetailResponse struct {
	SKU       string                           `json:"sku"`
	Name      string                           `json:"name"`
	Unit      string                           `json:"unit"`
	Locations []LocationStockBreakdownResponse `json:"locations"`
}

// LocationStockBreakdownResponse DTO разбивки остатков позиции по конкретному складу
type LocationStockBreakdownResponse struct {
	LocationID   string               `json:"location_id"`
	LocationName string               `json:"location_name"`
	TotalStock   decimal.Decimal      `json:"total_stock"`
	Batches      []BatchStockResponse `json:"batches"`
}

// BatchStockResponse DTO информации о партии товара на складе
type BatchStockResponse struct {
	Batch         string          `json:"batch"`
	Quantity      decimal.Decimal `json:"quantity"`
	ExpiryDate    *string         `json:"expiry_date,omitempty"`
	PurchasePrice decimal.Decimal `json:"purchase_price"`
}
