package calc

import (
	"fmt"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/shopspring/decimal"
)

// StockAlertInput входные данные позиции для анализа складских алертов
type StockAlertInput struct {
	Product             domain.Product
	LocationID          string
	CurrentStock        decimal.Decimal
	AvgDailyConsumption decimal.Decimal
	StockDays           *decimal.Decimal
	Batches             []domain.BatchStock
	DaysWithoutMovement int
	AsOfDate            time.Time
}

// EvaluateAlerts выявляет риски склада по требованиям ТЗ, делегируя проверки специализированным анализаторам.
func EvaluateAlerts(inputs []StockAlertInput, expiryThresholdDays int) []domain.Alert {
	if expiryThresholdDays <= 0 {
		expiryThresholdDays = 30
	}

	alerts := make([]domain.Alert, 0)
	for _, in := range inputs {
		if alert := checkStockoutRisk(in); alert != nil {
			alerts = append(alerts, *alert)
		}
		alerts = append(alerts, checkExpiryRisks(in, expiryThresholdDays)...)
		if alert := checkDeadStock(in); alert != nil {
			alerts = append(alerts, *alert)
		}
	}

	return alerts
}

// checkStockoutRisk проверяет условие: запаса в днях меньше, чем срок поставки
func checkStockoutRisk(in StockAlertInput) *domain.Alert {
	if in.StockDays == nil {
		return nil
	}

	leadTimeDec := decimal.NewFromInt(int64(in.Product.LeadTimeDays))
	if !in.StockDays.LessThan(leadTimeDec) {
		return nil
	}

	level := domain.AlertLevelWarning
	if in.StockDays.LessThan(leadTimeDec.Div(decimal.NewFromInt(2))) {
		level = domain.AlertLevelCritical
	}

	return &domain.Alert{
		Level:      level,
		Type:       domain.AlertTypeStockoutRisk,
		SKU:        in.Product.SKU,
		Name:       in.Product.Name,
		LocationID: in.LocationID,
		Message: fmt.Sprintf("Риск дефицита: текущего запаса (%s дн.) меньше срока поставки (%d дн.)",
			in.StockDays.String(), in.Product.LeadTimeDays),
		Metrics: map[string]interface{}{
			"stock_days":             in.StockDays.InexactFloat64(),
			"lead_time_days":         in.Product.LeadTimeDays,
			"current_stock":          in.CurrentStock.InexactFloat64(),
			"avg_daily_consumption": in.AvgDailyConsumption.InexactFloat64(),
		},
	}
}

// checkExpiryRisks проверяет партии с приближающимся или истекшим сроком годности
func checkExpiryRisks(in StockAlertInput, thresholdDays int) []domain.Alert {
	alerts := make([]domain.Alert, 0)

	for _, b := range in.Batches {
		if b.Quantity.LessThanOrEqual(decimal.Zero) || b.ExpiryDate == nil {
			continue
		}

		daysLeft := int(b.ExpiryDate.Sub(in.AsOfDate).Hours() / 24)
		if daysLeft > thresholdDays {
			continue
		}

		level := domain.AlertLevelWarning
		msg := fmt.Sprintf("Приближение срока годности: партия %s истекает %s (осталось %d дн., остаток %s %s)",
			b.BatchID, b.ExpiryDate.Format("02.01.2006"), daysLeft, b.Quantity.String(), in.Product.Unit)

		if daysLeft <= 0 {
			level = domain.AlertLevelCritical
			msg = fmt.Sprintf("Срок годности истек: партия %s просрочена %s (остаток %s %s подлежит списанию)",
				b.BatchID, b.ExpiryDate.Format("02.01.2006"), b.Quantity.String(), in.Product.Unit)
		} else if daysLeft <= 15 {
			level = domain.AlertLevelCritical
		}

		alerts = append(alerts, domain.Alert{
			Level:      level,
			Type:       domain.AlertTypeExpiryRisk,
			SKU:        in.Product.SKU,
			Name:       in.Product.Name,
			LocationID: in.LocationID,
			Message:    msg,
			Metrics: map[string]interface{}{
				"batch_id":    b.BatchID,
				"expiry_date": b.ExpiryDate.Format("2006-01-02"),
				"days_left":   daysLeft,
				"quantity":    b.Quantity.InexactFloat64(),
			},
		})
	}

	return alerts
}

// checkDeadStock проверяет наличие неликвидного остатка без расхода более 90 дней
func checkDeadStock(in StockAlertInput) *domain.Alert {
	if in.CurrentStock.LessThanOrEqual(decimal.Zero) {
		return nil
	}

	if !in.AvgDailyConsumption.IsZero() && in.DaysWithoutMovement < 90 {
		return nil
	}

	return &domain.Alert{
		Level:      domain.AlertLevelWarning,
		Type:       domain.AlertTypeDeadStock,
		SKU:        in.Product.SKU,
		Name:       in.Product.Name,
		LocationID: in.LocationID,
		Message: fmt.Sprintf("Отсутствие движения: позиция без расхода более 90 дней (остаток %s %s)",
			in.CurrentStock.String(), in.Product.Unit),
		Metrics: map[string]interface{}{
			"current_stock":          in.CurrentStock.InexactFloat64(),
			"days_without_movement": in.DaysWithoutMovement,
			"unit":                   in.Product.Unit,
		},
	}
}
