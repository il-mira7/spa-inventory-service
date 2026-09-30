package service

import (
	"context"
	"testing"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/il-mira7/spa-inventory-service/internal/repository/postgres"
	"github.com/shopspring/decimal"
)

// MockTxManager реализует прямой синхронный запуск без транзакции БД
type MockTxManager struct{}

func (m *MockTxManager) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// MockProductRepo
type MockProductRepo struct {
	Product       *domain.Product
	LocExists     bool
	LockedSKU     string
	UpsertedBatch *domain.Batch
}

func (m *MockProductRepo) GetProductBySKU(ctx context.Context, sku string) (*domain.Product, error) {
	if m.Product != nil && m.Product.SKU == sku {
		return m.Product, nil
	}
	return nil, domain.ErrNotFound
}
func (m *MockProductRepo) LockProductBySKU(ctx context.Context, sku string) error {
	m.LockedSKU = sku
	return nil
}
func (m *MockProductRepo) GetAllActiveProducts(ctx context.Context) ([]domain.Product, error) {
	if m.Product != nil {
		return []domain.Product{*m.Product}, nil
	}
	return nil, nil
}
func (m *MockProductRepo) LocationExists(ctx context.Context, locationID string) (bool, error) {
	return m.LocExists, nil
}
func (m *MockProductRepo) BatchExists(ctx context.Context, batchID, sku string) (bool, error) {
	return true, nil
}
func (m *MockProductRepo) GetBatch(ctx context.Context, batchID, sku string) (*domain.Batch, error) {
	return nil, nil
}
func (m *MockProductRepo) UpsertBatch(ctx context.Context, batch domain.Batch) error {
	m.UpsertedBatch = &batch
	return nil
}
func (m *MockProductRepo) GetLocations(ctx context.Context) ([]domain.Location, error) {
	return nil, nil
}
func (m *MockProductRepo) GetSuppliers(ctx context.Context) ([]domain.Supplier, error) {
	return nil, nil
}

// MockStockRepo
type MockStockRepo struct {
	CurrentStock  decimal.Decimal
	ActiveBatches []domain.BatchStock
	StockSummary  []domain.StockItem
	StockDetail   *domain.StockDetail
}

func (m *MockStockRepo) GetStockSummary(ctx context.Context, locationID *string) ([]domain.StockItem, error) {
	return m.StockSummary, nil
}
func (m *MockStockRepo) GetStockDetailBySKU(ctx context.Context, sku string) (*domain.StockDetail, error) {
	return m.StockDetail, nil
}
func (m *MockStockRepo) GetActiveBatchesBySKUAndLocation(ctx context.Context, sku, locationID string) ([]domain.BatchStock, error) {
	return m.ActiveBatches, nil
}
func (m *MockStockRepo) GetCurrentStock(ctx context.Context, sku, locationID string) (decimal.Decimal, error) {
	return m.CurrentStock, nil
}

// MockMovementRepo
type MockMovementRepo struct {
	ProcessedDocs   []domain.Movement
	InsertedRecords []domain.Movement
	MovementsWindow []domain.Movement
}

func (m *MockMovementRepo) InsertProcessedDocument(ctx context.Context, mov domain.Movement) error {
	m.ProcessedDocs = append(m.ProcessedDocs, mov)
	return nil
}
func (m *MockMovementRepo) InsertMovements(ctx context.Context, movements []domain.Movement) ([]int64, error) {
	m.InsertedRecords = append(m.InsertedRecords, movements...)
	ids := make([]int64, len(movements))
	for i := range movements {
		ids[i] = int64(100 + i)
	}
	return ids, nil
}
func (m *MockMovementRepo) GetMovements(ctx context.Context, filter postgres.MovementFilter, limit, offset int) ([]domain.Movement, int64, error) {
	return m.InsertedRecords, int64(len(m.InsertedRecords)), nil
}
func (m *MockMovementRepo) GetMovementsBySKUAndLocation(ctx context.Context, sku, locationID string) ([]domain.Movement, error) {
	return nil, nil
}
func (m *MockMovementRepo) GetMovementsInWindow(ctx context.Context, sku, locationID string, from, to time.Time) ([]domain.Movement, error) {
	return m.MovementsWindow, nil
}

// MockOrderRepo
type MockOrderRepo struct {
	IncomingQty decimal.Decimal
}

func (m *MockOrderRepo) GetIncomingQuantity(ctx context.Context, sku, locationID string) (decimal.Decimal, error) {
	return m.IncomingQty, nil
}
func (m *MockOrderRepo) GetOpenOrdersBySKUAndLocation(ctx context.Context, sku, locationID string) ([]domain.OpenOrder, error) {
	return nil, nil
}
func (m *MockOrderRepo) CreateOrder(ctx context.Context, order domain.OpenOrder) error {
	return nil
}

func TestMovementService_ProcessReceipt(t *testing.T) {
	prodRepo := &MockProductRepo{
		Product:   &domain.Product{SKU: "OIL-001", Name: "Масло", Unit: "л"},
		LocExists: true,
	}
	stockRepo := &MockStockRepo{
		CurrentStock: decimal.NewFromFloat(50.0),
	}
	movRepo := &MockMovementRepo{}

	svc := NewMovementService(&MockTxManager{}, movRepo, stockRepo, prodRepo)

	now := time.Now()
	expiry := now.AddDate(1, 0, 0)
	cmd := CreateMovementCommand{
		DocumentNo:    "DOC-REC-01",
		OperationDate: now,
		SKU:           "OIL-001",
		LocationID:    "MS-01",
		OperationType: domain.OperationReceipt,
		Quantity:      decimal.NewFromFloat(25.0),
		BatchID:       "BATCH-01",
		ExpiryDate:    &expiry,
		PurchasePrice: decimal.NewFromFloat(1200.0),
		Note:          "Поступление партии",
	}

	res, err := svc.ProcessMovement(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.CurrentStock.Cmp(decimal.NewFromFloat(50.0)) != 0 {
		t.Errorf("expected current_stock 50.0, got %s", res.CurrentStock.String())
	}
	if res.DocumentNo != "DOC-REC-01" {
		t.Errorf("expected document DOC-REC-01, got %s", res.DocumentNo)
	}
	if len(movRepo.InsertedRecords) != 1 {
		t.Fatalf("expected 1 movement inserted, got %d", len(movRepo.InsertedRecords))
	}
}

func TestMovementService_ProcessConsumption_FEFO_Splitting(t *testing.T) {
	prodRepo := &MockProductRepo{
		Product:   &domain.Product{SKU: "OIL-001", Name: "Масло", Unit: "л"},
		LocExists: true,
	}

	now := time.Now()
	exp1 := now.AddDate(0, 1, 0)
	exp2 := now.AddDate(0, 5, 0)

	stockRepo := &MockStockRepo{
		CurrentStock: decimal.NewFromFloat(40.39),
		ActiveBatches: []domain.BatchStock{
			{BatchID: "B1", SKU: "OIL-001", LocationID: "MS-01", Quantity: decimal.NewFromFloat(4.0), ExpiryDate: &exp1},
			{BatchID: "B2", SKU: "OIL-001", LocationID: "MS-01", Quantity: decimal.NewFromFloat(10.0), ExpiryDate: &exp2},
		},
	}
	movRepo := &MockMovementRepo{}

	svc := NewMovementService(&MockTxManager{}, movRepo, stockRepo, prodRepo)

	cmd := CreateMovementCommand{
		DocumentNo:    "DOC-CSM-01",
		OperationDate: now,
		SKU:           "OIL-001",
		LocationID:    "MS-01",
		OperationType: domain.OperationConsume,
		Quantity:      decimal.NewFromFloat(10.0),
		Note:          "Расход на спа",
	}

	res, err := svc.ProcessMovement(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем возврат пересчитанного остатка
	if res.CurrentStock.Cmp(decimal.NewFromFloat(40.39)) != 0 {
		t.Errorf("expected current stock 40.39, got %s", res.CurrentStock.String())
	}

	// Проверяем разбиение по партиям: 4.0 из B1 и 6.0 из B2
	if len(movRepo.InsertedRecords) != 2 {
		t.Fatalf("expected 2 movements inserted by FEFO, got %d", len(movRepo.InsertedRecords))
	}
	if movRepo.InsertedRecords[0].Quantity.Cmp(decimal.NewFromFloat(4.0)) != 0 || movRepo.InsertedRecords[0].BatchID != "B1" {
		t.Errorf("expected first movement 4.0 from B1, got %s from %s",
			movRepo.InsertedRecords[0].Quantity.String(), movRepo.InsertedRecords[0].BatchID)
	}
	if movRepo.InsertedRecords[1].Quantity.Cmp(decimal.NewFromFloat(6.0)) != 0 || movRepo.InsertedRecords[1].BatchID != "B2" {
		t.Errorf("expected second movement 6.0 from B2, got %s from %s",
			movRepo.InsertedRecords[1].Quantity.String(), movRepo.InsertedRecords[1].BatchID)
	}
}

func TestMovementService_ProcessWriteoff_AllowExpired(t *testing.T) {
	prodRepo := &MockProductRepo{
		Product:   &domain.Product{SKU: "SCR-003", Name: "Скраб", Unit: "кг"},
		LocExists: true,
	}

	pastDate := time.Now().AddDate(0, -1, 0) // Просроченная партия!
	stockRepo := &MockStockRepo{
		CurrentStock: decimal.NewFromFloat(5.0),
		ActiveBatches: []domain.BatchStock{
			{BatchID: "BATCH-EXPIRED", SKU: "SCR-003", LocationID: "MS-01", Quantity: decimal.NewFromFloat(10.0), ExpiryDate: &pastDate},
		},
	}
	movRepo := &MockMovementRepo{}

	svc := NewMovementService(&MockTxManager{}, movRepo, stockRepo, prodRepo)

	cmd := CreateMovementCommand{
		DocumentNo:    "DOC-WOF-01",
		OperationDate: time.Now(),
		SKU:           "SCR-003",
		LocationID:    "MS-01",
		OperationType: domain.OperationWriteoff,
		Quantity:      decimal.NewFromFloat(5.0),
		BatchID:       "BATCH-EXPIRED",
		Note:          "Списание просроченной косметики",
	}

	res, err := svc.ProcessMovement(context.Background(), cmd)
	if err != nil {
		t.Fatalf("writeoff must allow writing off expired batches, got error: %v", err)
	}

	if res.Quantity.Cmp(decimal.NewFromFloat(5.0)) != 0 {
		t.Errorf("expected quantity 5.0, got %s", res.Quantity.String())
	}
	if len(movRepo.InsertedRecords) != 1 {
		t.Fatalf("expected 1 movement inserted, got %d", len(movRepo.InsertedRecords))
	}
}

func TestMovementService_ValidationErrors(t *testing.T) {
	prodRepo := &MockProductRepo{
		Product:   &domain.Product{SKU: "OIL-001", Name: "Масло", Unit: "л"},
		LocExists: true,
	}
	svc := NewMovementService(&MockTxManager{}, &MockMovementRepo{}, &MockStockRepo{}, prodRepo)

	// 1. Пустой document_no должен возвращать domain.ErrInvalidDocumentNo
	_, err := svc.ProcessMovement(context.Background(), CreateMovementCommand{
		DocumentNo:    "",
		OperationDate: time.Now(),
		SKU:           "OIL-001",
		LocationID:    "MS-01",
		OperationType: domain.OperationReceipt,
		Quantity:      decimal.NewFromFloat(10.0),
	})
	if err != domain.ErrInvalidDocumentNo {
		t.Errorf("expected ErrInvalidDocumentNo, got %v", err)
	}

	// 2. Дата из будущего (> 1 мин) должна возвращать domain.ErrFutureDate
	_, err = svc.ProcessMovement(context.Background(), CreateMovementCommand{
		DocumentNo:    "DOC-FUTURE",
		OperationDate: time.Now().Add(5 * time.Minute),
		SKU:           "OIL-001",
		LocationID:    "MS-01",
		OperationType: domain.OperationReceipt,
		Quantity:      decimal.NewFromFloat(10.0),
	})
	if err != domain.ErrFutureDate {
		t.Errorf("expected ErrFutureDate, got %v", err)
	}
}

func TestForecastService_CalculateForecast_Contract(t *testing.T) {
	prodRepo := &MockProductRepo{
		Product: &domain.Product{
			SKU:               "OIL-001",
			Name:              "Массажное масло",
			Unit:              "л",
			PackageSize:       decimal.NewFromFloat(5.0),
			MinOrderQty:       decimal.NewFromFloat(25.0),
			LeadTimeDays:      7,
			LastPurchasePrice: decimal.NewFromFloat(1259.05),
		},
		LocExists: true,
	}

	asOf, _ := time.Parse("2006-01-02", "2026-09-16")
	from, _ := time.Parse("2006-01-02", "2026-10-01")
	to, _ := time.Parse("2006-01-02", "2026-12-31")

	// 122.58 л за 90 дней
	movements := []domain.Movement{
		{OperationType: domain.OperationConsume, Quantity: decimal.NewFromFloat(122.58), OperationDate: asOf.AddDate(0, 0, -10)},
	}

	stockRepo := &MockStockRepo{CurrentStock: decimal.NewFromFloat(50.39)}
	orderRepo := &MockOrderRepo{IncomingQty: decimal.NewFromFloat(20.0)}
	movRepo := &MockMovementRepo{MovementsWindow: movements}

	svc := NewForecastService(prodRepo, stockRepo, orderRepo, movRepo)

	res, err := svc.CalculateForecast(context.Background(), ForecastRequest{
		SKU:             "OIL-001",
		LocationID:      "MS-01",
		PeriodFrom:      from,
		PeriodTo:        to,
		SafetyStockDays: 14,
		AsOfDate:        &asOf,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.RecommendedPurchaseQty.Cmp(decimal.NewFromFloat(75.0)) != 0 {
		t.Errorf("expected recommended purchase 75.0, got %s", res.RecommendedPurchaseQty.String())
	}

	// Проверяем блок explanation строго по эталону ТЗ
	if len(res.Explanation.DataUsed) != 3 {
		t.Errorf("expected 3 data_used entries, got %d", len(res.Explanation.DataUsed))
	}
	if len(res.Explanation.Formulas) != 3 {
		t.Errorf("expected 3 formulas entries, got %d", len(res.Explanation.Formulas))
	}
	if res.Explanation.AsOf != "2026-09-16" {
		t.Errorf("expected as_of '2026-09-16', got %s", res.Explanation.AsOf)
	}
}

func TestAlertService_GetAlerts(t *testing.T) {
	prodRepo := &MockProductRepo{
		Product: &domain.Product{
			SKU:          "CRM-002",
			Name:         "Крем",
			Unit:         "кг",
			LeadTimeDays: 10,
		},
		LocExists: true,
	}

	stockDays := decimal.NewFromFloat(7.5)
	stockRepo := &MockStockRepo{
		StockSummary: []domain.StockItem{
			{
				SKU:                 "CRM-002",
				LocationID:          "MS-01",
				CurrentStock:        decimal.NewFromFloat(3.0),
				AvgDailyConsumption: decimal.NewFromFloat(0.4),
				StockDays:           &stockDays,
			},
		},
	}

	svc := NewAlertService(prodRepo, stockRepo)
	loc := "MS-01"
	alerts, err := svc.GetAlerts(context.Background(), &loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	alert := alerts[0]
	if alert.Type != domain.AlertTypeStockoutRisk {
		t.Errorf("expected stockout_risk, got %s", alert.Type)
	}
	if alert.Level != domain.AlertLevelWarning {
		t.Errorf("expected warning level, got %s", alert.Level)
	}
	if alert.Metrics["stock_days"] == nil || alert.Metrics["lead_time_days"] == nil {
		t.Errorf("expected metrics stock_days and lead_time_days, got %+v", alert.Metrics)
	}
}
