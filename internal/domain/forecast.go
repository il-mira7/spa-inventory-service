package domain

import (
	"github.com/shopspring/decimal"
)

// ForecastPeriod описывает временной интервал прогнозирования потребности
type ForecastPeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
	Days int    `json:"days"`
}

// ForecastExplanation содержит обоснование, прозрачность формул и исходные данные
type ForecastExplanation struct {
	DataUsed    []string `json:"data_used"`
	Formulas    []string `json:"formulas"`
	Assumptions []string `json:"assumptions"`
	AsOf        string   `json:"as_of"`
}

// ForecastWarning предупреждение в расчете прогноза
type ForecastWarning struct {
	Level   string `json:"level"` // info, warning, critical
	Message string `json:"message"`
}

// ForecastResult представляет полный ответ эндпоинта POST /api/forecast
type ForecastResult struct {
	SKU                    string              `json:"sku"`
	Name                   string              `json:"name"`
	Unit                   string              `json:"unit"`
	Location               string              `json:"location"`
	Period                 ForecastPeriod      `json:"period"`
	AvgDailyConsumption    decimal.Decimal     `json:"avg_daily_consumption"`
	ForecastDemand         decimal.Decimal     `json:"forecast_demand"`
	CurrentStock           decimal.Decimal     `json:"current_stock"`
	IncomingQty            decimal.Decimal     `json:"incoming_qty"`
	SafetyStock            decimal.Decimal     `json:"safety_stock"`
	ReorderPoint           decimal.Decimal     `json:"reorder_point"`
	RecommendedPurchaseQty decimal.Decimal     `json:"recommended_purchase_qty"`
	UnitPrice              decimal.Decimal     `json:"unit_price"`
	EstimatedCost          decimal.Decimal     `json:"estimated_cost"`
	RecommendedOrderDate   string              `json:"recommended_order_date"`
	StockoutDate           string              `json:"stockout_date"`
	Explanation            ForecastExplanation `json:"explanation"`
	Warnings               []ForecastWarning   `json:"warnings"`
}
