package calc

import (
	"fmt"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/shopspring/decimal"
)

// CalculateAvgDailyConsumption вычисляет среднедневной расход за последние 90 дней.
// Формула: расход за 90 дн. ÷ 90
func CalculateAvgDailyConsumption(movements []domain.Movement, asOf time.Time, windowDays int) decimal.Decimal {
	if windowDays <= 0 {
		windowDays = 90
	}

	startDate := asOf.AddDate(0, 0, -windowDays)
	totalConsumption := decimal.Zero

	for _, m := range movements {
		if m.OperationType == domain.OperationConsume {
			if (m.OperationDate.After(startDate) || m.OperationDate.Equal(startDate)) &&
				(m.OperationDate.Before(asOf) || m.OperationDate.Equal(asOf)) {
				totalConsumption = totalConsumption.Add(m.Quantity.Abs())
			}
		}
	}

	daysDec := decimal.NewFromInt(int64(windowDays))
	return totalConsumption.DivRound(daysDec, 4)
}

// CalculateStockDays рассчитывает запас товара в днях на основе текущего расхода.
func CalculateStockDays(stock decimal.Decimal, avgDailyConsumption decimal.Decimal) *decimal.Decimal {
	if avgDailyConsumption.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	days := stock.DivRound(avgDailyConsumption, 1)
	return &days
}

// CalculateSafetyStock рассчитывает страховой запас: среднедневной расход × страховой запас в днях
func CalculateSafetyStock(avgDailyConsumption decimal.Decimal, safetyStockDays int) decimal.Decimal {
	if safetyStockDays <= 0 {
		return decimal.Zero
	}
	return avgDailyConsumption.Mul(decimal.NewFromInt(int64(safetyStockDays))).Round(2)
}

// CalculateReorderPoint рассчитывает точку заказа (ROP): (среднедневной расход × срок поставки) + страховой запас
func CalculateReorderPoint(avgDailyConsumption decimal.Decimal, leadTimeDays int, safetyStock decimal.Decimal) decimal.Decimal {
	leadTimeDec := decimal.NewFromInt(int64(leadTimeDays))
	leadTimeDemand := avgDailyConsumption.Mul(leadTimeDec)
	return leadTimeDemand.Add(safetyStock).Round(1)
}

// CalculateRecommendedPurchase рассчитывает объем закупки с округлением до MOQ и кратности упаковки
func CalculateRecommendedPurchase(
	forecastDemand decimal.Decimal,
	safetyStock decimal.Decimal,
	currentStock decimal.Decimal,
	incomingQty decimal.Decimal,
	packageSize decimal.Decimal,
	minOrderQty decimal.Decimal,
) decimal.Decimal {
	rawQty := forecastDemand.Add(safetyStock).Sub(currentStock).Sub(incomingQty)
	if rawQty.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}

	qty := rawQty
	if minOrderQty.GreaterThan(decimal.Zero) && qty.LessThan(minOrderQty) {
		qty = minOrderQty
	}

	if packageSize.GreaterThan(decimal.Zero) {
		packages := qty.Div(packageSize).Ceil()
		qty = packages.Mul(packageSize)
	}

	return qty.Round(2)
}

// CalculateStockoutAndOrderDates вычисляет прогнозную дату окончания запасов и дату заказа
func CalculateStockoutAndOrderDates(
	asOf time.Time,
	currentStock decimal.Decimal,
	incomingQty decimal.Decimal,
	avgDailyConsumption decimal.Decimal,
	leadTimeDays int,
) (stockoutDate time.Time, recommendedOrderDate time.Time) {
	if avgDailyConsumption.LessThanOrEqual(decimal.Zero) {
		return time.Time{}, time.Time{}
	}

	totalAvailable := currentStock.Add(incomingQty)
	daysUntilStockout := int(totalAvailable.Div(avgDailyConsumption).IntPart())

	stockoutDate = asOf.AddDate(0, 0, daysUntilStockout)
	recommendedOrderDate = stockoutDate.AddDate(0, 0, -leadTimeDays)

	return stockoutDate, recommendedOrderDate
}

// ForecastParams входные данные для полного расчета прогноза
type ForecastParams struct {
	Product         domain.Product
	LocationID      string
	AsOfDate        time.Time
	PeriodFrom      time.Time
	PeriodTo        time.Time
	SafetyStockDays int
	Movements       []domain.Movement
	CurrentStock    decimal.Decimal
	IncomingQty     decimal.Decimal
	UnitPrice       decimal.Decimal
}

// CalculateForecast выполняет детерминированный расчет прогноза потребности и формирует объяснение
func CalculateForecast(params ForecastParams) *domain.ForecastResult {
	horizonDays := calculatePlanningHorizonDays(params.PeriodFrom, params.PeriodTo)
	avgDaily := CalculateAvgDailyConsumption(params.Movements, params.AsOfDate, 90)
	forecastDemand := avgDaily.Mul(decimal.NewFromInt(int64(horizonDays))).Round(1)
	safetyStock := CalculateSafetyStock(avgDaily, params.SafetyStockDays)
	reorderPoint := CalculateReorderPoint(avgDaily, params.Product.LeadTimeDays, safetyStock)

	recPurchaseQty := CalculateRecommendedPurchase(
		forecastDemand,
		safetyStock,
		params.CurrentStock,
		params.IncomingQty,
		params.Product.PackageSize,
		params.Product.MinOrderQty,
	)

	unitPrice := resolveUnitPrice(params.UnitPrice, params.Product.LastPurchasePrice)
	estimatedCost := recPurchaseQty.Mul(unitPrice).Round(2)

	stockoutDate, orderDate := CalculateStockoutAndOrderDates(
		params.AsOfDate,
		params.CurrentStock,
		params.IncomingQty,
		avgDaily,
		params.Product.LeadTimeDays,
	)

	return &domain.ForecastResult{
		SKU:      params.Product.SKU,
		Name:     params.Product.Name,
		Unit:     params.Product.Unit,
		Location: params.LocationID,
		Period: domain.ForecastPeriod{
			From: params.PeriodFrom.Format("2006-01-02"),
			To:   params.PeriodTo.Format("2006-01-02"),
			Days: horizonDays,
		},
		AvgDailyConsumption:    avgDaily.Round(3),
		ForecastDemand:         forecastDemand,
		CurrentStock:           params.CurrentStock.Round(2),
		IncomingQty:            params.IncomingQty.Round(1),
		SafetyStock:            safetyStock,
		ReorderPoint:           reorderPoint,
		RecommendedPurchaseQty: recPurchaseQty,
		UnitPrice:              unitPrice,
		EstimatedCost:          estimatedCost,
		RecommendedOrderDate:   formatDateOrEmpty(orderDate),
		StockoutDate:           formatDateOrEmpty(stockoutDate),
		Explanation:            buildExplanation(params.AsOfDate),
		Warnings:               evaluateForecastWarnings(params.CurrentStock, reorderPoint),
	}
}

func calculatePlanningHorizonDays(from, to time.Time) int {
	days := int(to.Sub(from).Hours()/24) + 1
	if days <= 0 {
		return 1
	}
	return days
}

func resolveUnitPrice(explicitPrice, fallbackPrice decimal.Decimal) decimal.Decimal {
	if explicitPrice.GreaterThan(decimal.Zero) {
		return explicitPrice
	}
	return fallbackPrice
}

func formatDateOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func evaluateForecastWarnings(currentStock, reorderPoint decimal.Decimal) []domain.ForecastWarning {
	warnings := make([]domain.ForecastWarning, 0)
	if currentStock.LessThan(reorderPoint) {
		warnings = append(warnings, domain.ForecastWarning{
			Level:   "warning",
			Message: "Остаток ниже точки заказа",
		})
	}
	return warnings
}

func buildExplanation(asOf time.Time) domain.ForecastExplanation {
	return domain.ForecastExplanation{
		DataUsed: []string{
			"Движение товара за последние 90 дней",
			fmt.Sprintf("Остатки по партиям на %s", asOf.Format("02.01.2006")),
			"Открытые заказы к поставке в периоде",
		},
		Formulas: []string{
			"средний расход/день = расход за 90 дн. ÷ 90",
			"прогноз = средний расход/день × дней в периоде",
			"объём = прогноз + страховой запас − остаток − поставки в пути",
		},
		Assumptions: []string{
			"Цена принята на уровне последней закупки.",
		},
		AsOf: asOf.Format("2006-01-02"),
	}
}
