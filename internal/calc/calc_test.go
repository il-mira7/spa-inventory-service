package calc_test

import (
	"testing"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/calc"
	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 1. Тест-кейс: Списание по FEFO (First Expired, First Out)
func TestAllocateFEFO_Success(t *testing.T) {
	asOf := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	exp1 := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC) // Ближайший срок
	exp2 := time.Date(2027, 7, 15, 0, 0, 0, 0, time.UTC)  // Дальний срок
	expOld := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) // Просроченная партия

	batches := []domain.BatchStock{
		{
			BatchID:       "BATCH-EXP2",
			SKU:           "OIL-001",
			LocationID:    "MS-01",
			Quantity:      decimal.NewFromFloat(50.0),
			ExpiryDate:    &exp2,
			PurchasePrice: decimal.NewFromFloat(1259.05),
			ReceivedDate:  asOf.AddDate(0, -2, 0),
		},
		{
			BatchID:       "BATCH-EXP1",
			SKU:           "OIL-001",
			LocationID:    "MS-01",
			Quantity:      decimal.NewFromFloat(20.0),
			ExpiryDate:    &exp1,
			PurchasePrice: decimal.NewFromFloat(1250.00),
			ReceivedDate:  asOf.AddDate(0, -4, 0),
		},
		{
			BatchID:       "BATCH-EXPIRED",
			SKU:           "OIL-001",
			LocationID:    "MS-01",
			Quantity:      decimal.NewFromFloat(10.0),
			ExpiryDate:    &expOld, // Истек 01.08.2026!
			PurchasePrice: decimal.NewFromFloat(1100.00),
			ReceivedDate:  asOf.AddDate(-1, 0, 0),
		},
	}

	// Запрашиваем 35 литров списания
	// Должно списаться:
	// - 20 литров целиком из BATCH-EXP1 (ближайший срок)
	// - 15 литров из BATCH-EXP2
	// - BATCH-EXPIRED должна быть полностью проигнорирована
	allocations, available, err := calc.AllocateFEFO(
		batches,
		decimal.NewFromFloat(35.0),
		asOf,
		"OIL-001",
		"MS-01",
	)

	require.NoError(t, err)
	assert.Equal(t, "70", available.String(), "Доступно должно быть 20 + 50 = 70 (без просроченной партии)")
	require.Len(t, allocations, 2)

	assert.Equal(t, "BATCH-EXP1", allocations[0].BatchID)
	assert.Equal(t, "20", allocations[0].AllocatedQty.String())

	assert.Equal(t, "BATCH-EXP2", allocations[1].BatchID)
	assert.Equal(t, "15", allocations[1].AllocatedQty.String())
}

// 2. Тест-кейс: Отказ при расходе больше доступного остатка (с указанием available_stock)
func TestAllocateFEFO_InsufficientStock(t *testing.T) {
	asOf := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	batches := []domain.BatchStock{
		{
			BatchID:    "BATCH-1",
			SKU:        "OIL-001",
			LocationID: "MS-01",
			Quantity:   decimal.NewFromFloat(15.5),
			ExpiryDate: &exp,
		},
	}

	// Запрашиваем 20 литров при доступных 15.5
	allocations, available, err := calc.AllocateFEFO(
		batches,
		decimal.NewFromFloat(20.0),
		asOf,
		"OIL-001",
		"MS-01",
	)

	require.Error(t, err)
	assert.Nil(t, allocations)
	assert.Equal(t, "15.5", available.String())

	insufficientErr, ok := domain.IsInsufficientStock(err)
	require.True(t, ok, "Ошибка должна приводиться к domain.InsufficientStockError")
	assert.Equal(t, "OIL-001", insufficientErr.SKU)
	assert.Equal(t, "MS-01", insufficientErr.LocationID)
	assert.Equal(t, "20", insufficientErr.Requested.String())
	assert.Equal(t, "15.5", insufficientErr.AvailableStock.String())
}

// 3. Тест-кейс: Расчёт остатка из последовательности движений (Ledger Pattern)
func TestAggregateStock(t *testing.T) {
	now := time.Now()
	movements := []domain.Movement{
		{OperationType: domain.OperationReceipt, Quantity: decimal.NewFromFloat(100.0), OperationDate: now},
		{OperationType: domain.OperationConsume, Quantity: decimal.NewFromFloat(25.5), OperationDate: now},
		{OperationType: domain.OperationReturn, Quantity: decimal.NewFromFloat(5.0), OperationDate: now},
		{OperationType: domain.OperationWriteoff, Quantity: decimal.NewFromFloat(4.5), OperationDate: now},
		{OperationType: domain.OperationCorrection, Quantity: decimal.NewFromFloat(-5.0), OperationDate: now}, // корректировка в минус
	}

	// 100 - 25.5 + 5 - 4.5 - 5 = 70.0
	stock := calc.AggregateStock(movements)
	assert.Equal(t, "70", stock.String())
}

// 4. Тест-кейс: Округление рекомендуемого объёма закупки до упаковки и минимальной партии (MOQ)
func TestCalculateRecommendedPurchase(t *testing.T) {
	tests := []struct {
		name           string
		forecastDemand decimal.Decimal
		safetyStock    decimal.Decimal
		currentStock   decimal.Decimal
		incomingQty    decimal.Decimal
		packageSize    decimal.Decimal
		minOrderQty    decimal.Decimal
		expected       decimal.Decimal
	}{
		{
			name:           "Эталонный кейс из ТЗ (OIL-001)",
			forecastDemand: decimal.NewFromFloat(125.3),
			safetyStock:    decimal.NewFromFloat(19.07),
			currentStock:   decimal.NewFromFloat(50.39),
			incomingQty:    decimal.NewFromFloat(20.0),
			packageSize:    decimal.NewFromFloat(5.0),
			minOrderQty:    decimal.NewFromFloat(25.0),
			// raw = 125.3 + 19.07 - 50.39 - 20.0 = 73.98
			// max(73.98, 25.0) = 73.98
			// ceil(73.98 / 5.0) * 5.0 = 15 * 5.0 = 75.0
			expected: decimal.NewFromFloat(75.0),
		},
		{
			name:           "Ограничение минимальной партией поставщика (MOQ)",
			forecastDemand: decimal.NewFromFloat(10.0),
			safetyStock:    decimal.NewFromFloat(5.0),
			currentStock:   decimal.NewFromFloat(3.0),
			incomingQty:    decimal.NewFromFloat(0.0),
			packageSize:    decimal.NewFromFloat(2.0),
			minOrderQty:    decimal.NewFromFloat(20.0),
			// raw = 10 + 5 - 3 = 12.0
			// max(12.0, 20.0) = 20.0
			// ceil(20 / 2) * 2 = 20.0
			expected: decimal.NewFromFloat(20.0),
		},
		{
			name:           "Остаток полностью покрывает потребность (закупка не требуется)",
			forecastDemand: decimal.NewFromFloat(50.0),
			safetyStock:    decimal.NewFromFloat(10.0),
			currentStock:   decimal.NewFromFloat(80.0),
			incomingQty:    decimal.NewFromFloat(0.0),
			packageSize:    decimal.NewFromFloat(5.0),
			minOrderQty:    decimal.NewFromFloat(25.0),
			// raw = 50 + 10 - 80 = -20 <= 0 -> 0
			expected: decimal.Zero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := calc.CalculateRecommendedPurchase(
				tt.forecastDemand,
				tt.safetyStock,
				tt.currentStock,
				tt.incomingQty,
				tt.packageSize,
				tt.minOrderQty,
			)
			assert.True(t, tt.expected.Equal(actual), "expected %s, got %s", tt.expected.String(), actual.String())
		})
	}
}

// 5. Тест-кейс: Расчёт точки заказа (ROP), даты дефицита и даты заказа
func TestCalculateReorderPointAndDates(t *testing.T) {
	avgDaily := decimal.NewFromFloat(1.362)
	leadTimeDays := 7
	safetyStock := decimal.NewFromFloat(19.07)

	// ROP = (1.362 * 7) + 19.07 = 9.534 + 19.07 = 28.604 -> 28.6
	rop := calc.CalculateReorderPoint(avgDaily, leadTimeDays, safetyStock)
	assert.Equal(t, "28.6", rop.String(), "Точка заказа должна быть 28.6")

	asOf := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	currentStock := decimal.NewFromFloat(50.39)
	incomingQty := decimal.NewFromFloat(20.0)

	// (50.39 + 20.0) / 1.362 = 70.39 / 1.362 = 51 день
	// 15.09.2026 + 51 день -> 01.11.2026
	// Дата заказа = 01.11.2026 - 7 дней -> 25.10.2026
	stockoutDate, orderDate := calc.CalculateStockoutAndOrderDates(
		asOf,
		currentStock,
		incomingQty,
		avgDaily,
		leadTimeDays,
	)

	assert.Equal(t, "2026-11-05", stockoutDate.Format("2006-01-02"))
	assert.Equal(t, "2026-10-29", orderDate.Format("2006-01-02"))
}

// 6. Тест-кейс: Полный расчет прогноза POST /api/forecast (воспроизведение эталонного ответа ТЗ)
func TestCalculateForecast_ReferenceContract(t *testing.T) {
	asOf := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	periodFrom := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	periodTo := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC) // 92 дня

	product := domain.Product{
		SKU:               "OIL-001",
		Name:              "Массажное масло базовое (миндаль)",
		Unit:              "л",
		LeadTimeDays:      7,
		PackageSize:       decimal.NewFromFloat(5.0),
		MinOrderQty:       decimal.NewFromFloat(25.0),
		LastPurchasePrice: decimal.NewFromFloat(1259.05),
	}

	// 122.58 л расхода за 90 дней -> 1.362 л/день
	movements := []domain.Movement{
		{
			OperationType: domain.OperationConsume,
			Quantity:      decimal.NewFromFloat(122.58),
			OperationDate: asOf.AddDate(0, 0, -30),
		},
	}

	params := calc.ForecastParams{
		Product:         product,
		LocationID:      "MS-01",
		AsOfDate:        asOf,
		PeriodFrom:      periodFrom,
		PeriodTo:        periodTo,
		SafetyStockDays: 14,
		Movements:       movements,
		CurrentStock:    decimal.NewFromFloat(50.39),
		IncomingQty:     decimal.NewFromFloat(20.0),
		UnitPrice:       decimal.NewFromFloat(1259.05),
	}

	res := calc.CalculateForecast(params)

	assert.Equal(t, "OIL-001", res.SKU)
	assert.Equal(t, "MS-01", res.Location)
	assert.Equal(t, 92, res.Period.Days)
	assert.Equal(t, "1.362", res.AvgDailyConsumption.String())
	assert.Equal(t, "125.3", res.ForecastDemand.String())
	assert.Equal(t, "50.39", res.CurrentStock.String())
	assert.Equal(t, "20", res.IncomingQty.String())
	assert.Equal(t, "19.07", res.SafetyStock.String())
	assert.Equal(t, "28.6", res.ReorderPoint.String())
	assert.Equal(t, "75", res.RecommendedPurchaseQty.String())
	assert.Equal(t, "1259.05", res.UnitPrice.String())
	assert.Equal(t, "94428.75", res.EstimatedCost.String())
	assert.Equal(t, "2026-10-29", res.RecommendedOrderDate)
	assert.Equal(t, "2026-11-05", res.StockoutDate)
	assert.NotEmpty(t, res.Explanation.DataUsed)
	assert.NotEmpty(t, res.Explanation.Formulas)
}

// 7. Тест-кейс: Выявление алертов склада (Дефицит, Срок годности, Неликвид)
func TestEvaluateAlerts(t *testing.T) {
	asOf := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	stockDays75 := decimal.NewFromFloat(7.5)
	stockDays30 := decimal.NewFromFloat(30.0)
	expSoon := asOf.AddDate(0, 0, 15) // через 15 дней

	inputs := []calc.StockAlertInput{
		{
			// Позиция 1: Риск дефицита (CRM-002: запас 7.5 дн при сроке поставки 10 дн)
			Product: domain.Product{
				SKU:          "CRM-002",
				Name:         "Крем манго",
				LeadTimeDays: 10,
				Unit:         "кг",
			},
			LocationID:          "MS-01",
			CurrentStock:        decimal.NewFromFloat(3.0),
			AvgDailyConsumption: decimal.NewFromFloat(0.4),
			StockDays:           &stockDays75,
			AsOfDate:            asOf,
		},
		{
			// Позиция 2: Приближение срока годности (SCR-003: партия истекает через 15 дней)
			Product: domain.Product{
				SKU:          "SCR-003",
				Name:         "Скраб детокс",
				LeadTimeDays: 5,
				Unit:         "кг",
			},
			LocationID:          "MS-01",
			CurrentStock:        decimal.NewFromFloat(10.0),
			AvgDailyConsumption: decimal.NewFromFloat(0.3),
			StockDays:           &stockDays30,
			Batches: []domain.BatchStock{
				{
					BatchID:    "BATCH-SCR-EXP",
					Quantity:   decimal.NewFromFloat(5.0),
					ExpiryDate: &expSoon,
				},
			},
			AsOfDate: asOf,
		},
		{
			// Позиция 3: Неликвид / dead stock (OIL-005: остаток есть, расхода за 90 дней нет)
			Product: domain.Product{
				SKU:          "OIL-005",
				Name:         "Эфирное масло",
				LeadTimeDays: 14,
				Unit:         "фл",
			},
			LocationID:          "MS-01",
			CurrentStock:        decimal.NewFromFloat(15.0),
			AvgDailyConsumption: decimal.Zero,
			DaysWithoutMovement: 95,
			AsOfDate:            asOf,
		},
	}

	alerts := calc.EvaluateAlerts(inputs, 30)
	require.Len(t, alerts, 3)

	assert.Equal(t, domain.AlertTypeStockoutRisk, alerts[0].Type)
	assert.Equal(t, "CRM-002", alerts[0].SKU)

	assert.Equal(t, domain.AlertTypeExpiryRisk, alerts[1].Type)
	assert.Equal(t, "SCR-003", alerts[1].SKU)

	assert.Equal(t, domain.AlertTypeDeadStock, alerts[2].Type)
	assert.Equal(t, "OIL-005", alerts[2].SKU)
}
