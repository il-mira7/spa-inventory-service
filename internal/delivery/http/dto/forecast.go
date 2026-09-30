package dto

import (
	"fmt"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/shopspring/decimal"
)

// ForecastRequestDTO DTO запроса расчета потребности и плана закупок POST /api/forecast
type ForecastRequestDTO struct {
	SKU             string   `json:"sku"`
	Location        string   `json:"location"`
	LocationID      string   `json:"location_id,omitempty"`
	PeriodFrom      *string  `json:"period_from,omitempty"`
	PeriodTo        *string  `json:"period_to,omitempty"`
	HorizonDays     *int     `json:"horizon_days,omitempty"`
	HorizonMonths   *int     `json:"horizon_months,omitempty"`
	SafetyStockDays *int     `json:"safety_stock_days,omitempty"`
	UnitPrice       *float64 `json:"unit_price,omitempty"`
	AsOfDate        *string  `json:"as_of_date,omitempty"`
}

// Validate проверяет корректность входных параметров расчета прогноза
func (r *ForecastRequestDTO) Validate() error {
	if r.SKU == "" {
		return fmt.Errorf("sku is required: %w", domain.ErrInvalidOperation)
	}
	if r.LocationID == "" {
		r.LocationID = r.Location
	}
	if r.LocationID == "" {
		return fmt.Errorf("location is required: %w", domain.ErrInvalidOperation)
	}

	// Должен быть задан либо диапазон дат, либо горизонт в днях/месяцах
	hasDirectDates := (r.PeriodFrom != nil && *r.PeriodFrom != "") && (r.PeriodTo != nil && *r.PeriodTo != "")
	hasHorizon := (r.HorizonDays != nil && *r.HorizonDays > 0) || (r.HorizonMonths != nil && *r.HorizonMonths > 0)

	if !hasDirectDates && !hasHorizon {
		return fmt.Errorf("either period (period_from, period_to) or horizon (horizon_days/horizon_months) is required: %w", domain.ErrInvalidOperation)
	}

	if hasDirectDates {
		from, to, err := r.ParsePeriod()
		if err != nil {
			return err
		}
		if from.After(to) {
			return fmt.Errorf("period_from cannot be after period_to: %w", domain.ErrInvalidOperation)
		}
	}

	return nil
}

// ParsePeriod вычисляет начало и конец расчетного периода прогноза с поддержкой horizon_days и horizon_months
func (r *ForecastRequestDTO) ParsePeriod() (time.Time, time.Time, error) {
	baseDate := time.Now().UTC()
	if r.AsOfDate != nil && *r.AsOfDate != "" {
		if t, err := parseDate(*r.AsOfDate); err == nil {
			baseDate = t
		}
	}

	// 1. Прямые даты period_from и period_to
	if r.PeriodFrom != nil && *r.PeriodFrom != "" && r.PeriodTo != nil && *r.PeriodTo != "" {
		from, err := parseDate(*r.PeriodFrom)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid period_from: %w", domain.ErrInvalidOperation)
		}
		to, err := parseDate(*r.PeriodTo)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid period_to: %w", domain.ErrInvalidOperation)
		}
		if from.After(to) {
			return time.Time{}, time.Time{}, fmt.Errorf("period_from cannot be after period_to: %w", domain.ErrInvalidOperation)
		}
		return from, to, nil
	}

	// 2. Горизонт в днях
	if r.HorizonDays != nil && *r.HorizonDays > 0 {
		from := baseDate
		if r.PeriodFrom != nil && *r.PeriodFrom != "" {
			if t, err := parseDate(*r.PeriodFrom); err == nil {
				from = t
			}
		}
		to := from.AddDate(0, 0, *r.HorizonDays)
		return from, to, nil
	}

	// 3. Горизонт в месяцах
	if r.HorizonMonths != nil && *r.HorizonMonths > 0 {
		from := baseDate
		if r.PeriodFrom != nil && *r.PeriodFrom != "" {
			if t, err := parseDate(*r.PeriodFrom); err == nil {
				from = t
			}
		}
		to := from.AddDate(0, *r.HorizonMonths, 0)
		return from, to, nil
	}

	return time.Time{}, time.Time{}, fmt.Errorf("unable to determine forecast period: %w", domain.ErrInvalidOperation)
}

// ParseAsOfDate парсит опциональную дату состояния остатков
func (r *ForecastRequestDTO) ParseAsOfDate() (*time.Time, error) {
	if r.AsOfDate == nil || *r.AsOfDate == "" {
		return nil, nil
	}
	t, err := parseDate(*r.AsOfDate)
	if err != nil {
		return nil, fmt.Errorf("invalid as_of_date: %w", domain.ErrInvalidOperation)
	}
	return &t, nil
}

func parseDate(s string) (time.Time, error) {
	layouts := []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format %s", s)
}

// ForecastResponseDTO DTO эталонного ответа прогнозирования POST /api/forecast
type ForecastResponseDTO struct {
	SKU                    string                     `json:"sku"`
	Name                   string                     `json:"name"`
	Unit                   string                     `json:"unit"`
	Location               string                     `json:"location"`
	Period                 domain.ForecastPeriod      `json:"period"`
	AvgDailyConsumption    decimal.Decimal            `json:"avg_daily_consumption"`
	ForecastDemand         decimal.Decimal            `json:"forecast_demand"`
	CurrentStock           decimal.Decimal            `json:"current_stock"`
	IncomingQty            decimal.Decimal            `json:"incoming_qty"`
	SafetyStock            decimal.Decimal            `json:"safety_stock"`
	ReorderPoint           decimal.Decimal            `json:"reorder_point"`
	RecommendedPurchaseQty decimal.Decimal            `json:"recommended_purchase_qty"`
	UnitPrice              decimal.Decimal            `json:"unit_price"`
	EstimatedCost          decimal.Decimal            `json:"estimated_cost"`
	RecommendedOrderDate   string                     `json:"recommended_order_date"`
	StockoutDate           string                     `json:"stockout_date"`
	Explanation            domain.ForecastExplanation `json:"explanation"`
	Warnings               []domain.ForecastWarning   `json:"warnings"`
}
