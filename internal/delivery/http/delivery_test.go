package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/config"
	deliveryHTTP "github.com/il-mira7/spa-inventory-service/internal/delivery/http"
	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/dto"
	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/handlers"
	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/il-mira7/spa-inventory-service/internal/repository/postgres"
	"github.com/il-mira7/spa-inventory-service/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock Pinger ---
type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

// --- Mock MovementService ---
type mockMovementService struct {
	processFunc func(ctx context.Context, cmd service.CreateMovementCommand) (*domain.MovementResult, error)
	listFunc    func(ctx context.Context, filter postgres.MovementFilter, limit, offset int) ([]domain.Movement, int64, error)
}

func (m *mockMovementService) ProcessMovement(ctx context.Context, cmd service.CreateMovementCommand) (*domain.MovementResult, error) {
	if m.processFunc != nil {
		return m.processFunc(ctx, cmd)
	}
	return nil, nil
}

func (m *mockMovementService) GetMovements(ctx context.Context, filter postgres.MovementFilter, limit, offset int) ([]domain.Movement, int64, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, filter, limit, offset)
	}
	return nil, 0, nil
}

// --- Mock StockService ---
type mockStockService struct {
	summaryFunc func(ctx context.Context, locationID *string) ([]domain.StockItem, error)
	detailFunc  func(ctx context.Context, sku string) (*domain.StockDetail, error)
}

func (m *mockStockService) GetStockSummary(ctx context.Context, locationID *string) ([]domain.StockItem, error) {
	if m.summaryFunc != nil {
		return m.summaryFunc(ctx, locationID)
	}
	return nil, nil
}

func (m *mockStockService) GetStockDetail(ctx context.Context, sku string) (*domain.StockDetail, error) {
	if m.detailFunc != nil {
		return m.detailFunc(ctx, sku)
	}
	return nil, nil
}

// --- Mock ForecastService ---
type mockForecastService struct {
	forecastFunc func(ctx context.Context, req service.ForecastRequest) (*domain.ForecastResult, error)
}

func (m *mockForecastService) CalculateForecast(ctx context.Context, req service.ForecastRequest) (*domain.ForecastResult, error) {
	if m.forecastFunc != nil {
		return m.forecastFunc(ctx, req)
	}
	return nil, nil
}

// --- Mock AlertService ---
type mockAlertService struct {
	alertsFunc func(ctx context.Context, locationID *string) ([]domain.Alert, error)
}

func (m *mockAlertService) GetAlerts(ctx context.Context, locationID *string) ([]domain.Alert, error) {
	if m.alertsFunc != nil {
		return m.alertsFunc(ctx, locationID)
	}
	return nil, nil
}

// --- Mock ProductRepository ---
type mockProductRepo struct {
	postgres.ProductRepository
	locationsFunc func(ctx context.Context) ([]domain.Location, error)
	suppliersFunc func(ctx context.Context) ([]domain.Supplier, error)
	productsFunc  func(ctx context.Context) ([]domain.Product, error)
}

func (m *mockProductRepo) GetLocations(ctx context.Context) ([]domain.Location, error) {
	if m.locationsFunc != nil {
		return m.locationsFunc(ctx)
	}
	return []domain.Location{}, nil
}

func (m *mockProductRepo) GetSuppliers(ctx context.Context) ([]domain.Supplier, error) {
	if m.suppliersFunc != nil {
		return m.suppliersFunc(ctx)
	}
	return []domain.Supplier{}, nil
}

func (m *mockProductRepo) GetAllActiveProducts(ctx context.Context) ([]domain.Product, error) {
	if m.productsFunc != nil {
		return m.productsFunc(ctx)
	}
	return []domain.Product{}, nil
}

// helper to build test router
type testHarness struct {
	pinger      *mockPinger
	movementSvc *mockMovementService
	stockSvc    *mockStockService
	forecastSvc *mockForecastService
	alertSvc    *mockAlertService
	productRepo *mockProductRepo
	server      *httptest.Server
}

func setupTestHarness(t *testing.T) *testHarness {
	h := &testHarness{
		pinger:      &mockPinger{},
		movementSvc: &mockMovementService{},
		stockSvc:    &mockStockService{},
		forecastSvc: &mockForecastService{},
		alertSvc:    &mockAlertService{},
		productRepo: &mockProductRepo{},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		AppEnv:      "test",
		HTTPPort:    8080,
		HTTPTimeout: 5 * time.Second,
	}

	healthH := handlers.NewHealthHandlerWithPinger(h.pinger)
	movementH := handlers.NewMovementHandler(h.movementSvc)
	stockH := handlers.NewStockHandler(h.stockSvc)
	forecastH := handlers.NewForecastHandler(h.forecastSvc)
	alertH := handlers.NewAlertHandler(h.alertSvc)
	refH := handlers.NewReferenceHandler(h.productRepo)

	router := deliveryHTTP.NewRouter(deliveryHTTP.RouterParams{
		Config:          cfg,
		Logger:          logger,
		HealthHandler:   healthH,
		MovementHandler: movementH,
		StockHandler:    stockH,
		ForecastHandler: forecastH,
		AlertHandler:    alertH,
		RefHandler:      refH,
	})

	h.server = httptest.NewServer(router)
	t.Cleanup(func() {
		h.server.Close()
	})

	return h
}

// ======================== TESTS ========================

// 1. Тестирование ошибки нехватки остатка с полями available_stock и requested точно по ТЗ
func TestInsufficientStock_ErrorResponse(t *testing.T) {
	h := setupTestHarness(t)
	h.movementSvc.processFunc = func(ctx context.Context, cmd service.CreateMovementCommand) (*domain.MovementResult, error) {
		return nil, &domain.InsufficientStockError{
			SKU:            "OIL-001",
			Requested:      decimal.NewFromFloat(10.0),
			AvailableStock: decimal.NewFromFloat(4.5),
		}
	}

	payload := map[string]any{
		"document_no":    "DOC-OUT-001",
		"operation_date": time.Now().UTC().Format(time.RFC3339),
		"sku":            "OIL-001",
		"location":       "loc-1",
		"operation_type": "consume",
		"quantity":       10.0,
	}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(h.server.URL+"/api/movements", "application/json", bytes.NewReader(data))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	var errResp dto.ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnprocessableEntity, errResp.Code)
	assert.Equal(t, "OIL-001", errResp.SKU)
	require.NotNil(t, errResp.AvailableStock, "available_stock field must be present")
	assert.Equal(t, 4.5, *errResp.AvailableStock)
	require.NotNil(t, errResp.Requested, "requested field must be present")
	assert.Equal(t, 10.0, *errResp.Requested)
}

// 2. Тестирование редиректа с GET /docs на /swagger/
func TestDocs_Redirect(t *testing.T) {
	h := setupTestHarness(t)

	// Клиент без автоматического перехода по редиректам
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(h.server.URL + "/docs")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
	assert.Equal(t, "/swagger/", resp.Header.Get("Location"))
}

// 3. Тестирование ForecastRequestDTO: прямые даты и горизонты horizon_days / horizon_months
func TestForecastRequestDTO_Horizons(t *testing.T) {
	h := setupTestHarness(t)

	t.Run("Горизонт в днях (horizon_days)", func(t *testing.T) {
		called := false
		h.forecastSvc.forecastFunc = func(ctx context.Context, req service.ForecastRequest) (*domain.ForecastResult, error) {
			called = true
			assert.Equal(t, "OIL-001", req.SKU)
			assert.Equal(t, "loc-1", req.LocationID)
			// Проверяем, что период вычислен: to - from = 30 дней
			days := int(req.PeriodTo.Sub(req.PeriodFrom).Hours() / 24)
			assert.Equal(t, 30, days)

			return &domain.ForecastResult{
				SKU:                    "OIL-001",
				Location:               "loc-1",
				AvgDailyConsumption:    decimal.NewFromFloat(0.5),
				ForecastDemand:         decimal.NewFromFloat(15.0),
				CurrentStock:           decimal.NewFromFloat(5.0),
				SafetyStock:            decimal.NewFromFloat(7.0),
				ReorderPoint:           decimal.NewFromFloat(7.0),
				RecommendedPurchaseQty: decimal.NewFromFloat(17.0),
				Explanation: domain.ForecastExplanation{
					Formulas: []string{"formula"},
				},
			}, nil
		}

		payload := map[string]any{
			"sku":          "OIL-001",
			"location_id":  "loc-1",
			"horizon_days": 30,
			"as_of_date":   "2026-10-01",
		}
		data, _ := json.Marshal(payload)

		resp, err := http.Post(h.server.URL+"/api/forecast", "application/json", bytes.NewReader(data))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.True(t, called)
	})

	t.Run("Горизонт в месяцах (horizon_months)", func(t *testing.T) {
		called := false
		h.forecastSvc.forecastFunc = func(ctx context.Context, req service.ForecastRequest) (*domain.ForecastResult, error) {
			called = true
			assert.Equal(t, "OIL-001", req.SKU)
			assert.Equal(t, "loc-1", req.LocationID)
			assert.Equal(t, 2026, req.PeriodFrom.Year())
			assert.Equal(t, time.Month(10), req.PeriodFrom.Month())
			assert.Equal(t, time.Month(12), req.PeriodTo.Month()) // 10 + 2 months = 12

			return &domain.ForecastResult{
				SKU:                    "OIL-001",
				Location:               "loc-1",
				AvgDailyConsumption:    decimal.NewFromFloat(0.5),
				ForecastDemand:         decimal.NewFromFloat(30.0),
				CurrentStock:           decimal.NewFromFloat(5.0),
				SafetyStock:            decimal.NewFromFloat(7.0),
				ReorderPoint:           decimal.NewFromFloat(7.0),
				RecommendedPurchaseQty: decimal.NewFromFloat(32.0),
				Explanation: domain.ForecastExplanation{
					Formulas: []string{"formula"},
				},
			}, nil
		}

		payload := map[string]any{
			"sku":            "OIL-001",
			"location":       "loc-1",
			"horizon_months": 2,
			"as_of_date":     "2026-10-01",
		}
		data, _ := json.Marshal(payload)

		resp, err := http.Post(h.server.URL+"/api/forecast", "application/json", bytes.NewReader(data))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.True(t, called)
	})
}

// 4. Тестирование фильтров GET /api/movements: sku, location (location_id), operation_type (type), from_date (from), to_date (to), limit, offset
func TestMovements_FilterParams(t *testing.T) {
	h := setupTestHarness(t)
	now := time.Now().UTC()

	t.Run("Фильтрация по location_id, operation_type, from_date, to_date", func(t *testing.T) {
		h.movementSvc.listFunc = func(ctx context.Context, filter postgres.MovementFilter, limit, offset int) ([]domain.Movement, int64, error) {
			assert.Equal(t, "OIL-001", *filter.SKU)
			assert.Equal(t, "loc-2", *filter.LocationID)
			assert.Equal(t, domain.OperationReceipt, *filter.OperationType)
			require.NotNil(t, filter.FromDate)
			assert.Equal(t, "2026-09-01", filter.FromDate.Format("2006-01-02"))
			require.NotNil(t, filter.ToDate)
			assert.Equal(t, "2026-09-30", filter.ToDate.Format("2006-01-02"))
			assert.Equal(t, 25, limit)
			assert.Equal(t, 50, offset)

			return []domain.Movement{
				{
					ID:            1,
					DocumentNo:    "DOC-FILTER-001",
					OperationDate: now,
					SKU:           "OIL-001",
					LocationID:    "loc-2",
					OperationType: domain.OperationReceipt,
					Quantity:      decimal.NewFromFloat(10),
					CreatedAt:     now,
				},
			}, 1, nil
		}

		url := h.server.URL + "/api/movements?sku=OIL-001&location_id=loc-2&operation_type=receipt&from_date=2026-09-01&to_date=2026-09-30&limit=25&offset=50"
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var listResp dto.MovementsListResponse
		err = json.NewDecoder(resp.Body).Decode(&listResp)
		require.NoError(t, err)
		assert.Equal(t, int64(1), listResp.Total)
		assert.Equal(t, 25, listResp.Limit)
		assert.Equal(t, 50, listResp.Offset)
	})

	t.Run("Фильтрация по альтернативным ключам: location, type, from, to", func(t *testing.T) {
		h.movementSvc.listFunc = func(ctx context.Context, filter postgres.MovementFilter, limit, offset int) ([]domain.Movement, int64, error) {
			assert.Equal(t, "loc-1", *filter.LocationID)
			assert.Equal(t, domain.OperationConsume, *filter.OperationType)
			require.NotNil(t, filter.FromDate)
			assert.Equal(t, "2026-08-01", filter.FromDate.Format("2006-01-02"))
			require.NotNil(t, filter.ToDate)
			assert.Equal(t, "2026-08-31", filter.ToDate.Format("2006-01-02"))

			return []domain.Movement{}, 0, nil
		}

		url := h.server.URL + "/api/movements?location=loc-1&type=consume&from=2026-08-01&to=2026-08-31"
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// 5. Тестирование раздачи Swagger UI через embed.FS
func TestSwagger_EmbedFS(t *testing.T) {
	h := setupTestHarness(t)

	t.Run("GET /swagger/ возвращает HTML интерфейс из embed.FS", func(t *testing.T) {
		resp, err := http.Get(h.server.URL + "/swagger/")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
		bodyBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(bodyBytes), "SwaggerUIBundle")
		assert.Contains(t, string(bodyBytes), "/swagger/openapi.yaml")
	})

	t.Run("GET /swagger/openapi.yaml возвращает спецификацию из embed.FS", func(t *testing.T) {
		resp, err := http.Get(h.server.URL + "/swagger/openapi.yaml")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "application/yaml")
		bodyBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(bodyBytes), "openapi: 3.0.3")
		assert.Contains(t, string(bodyBytes), "Mountain & Sea Spa")
	})
}

// 6. Тестирование POST /api/movements с возвратом актуального остатка и идемпотентностью
func TestMovements_Create_Operations(t *testing.T) {
	h := setupTestHarness(t)
	now := time.Now().UTC()

	t.Run("201 Created на корректное оприходование с возвратом current_stock", func(t *testing.T) {
		h.movementSvc.processFunc = func(ctx context.Context, cmd service.CreateMovementCommand) (*domain.MovementResult, error) {
			assert.Equal(t, "DOC-REC-001", cmd.DocumentNo)
			return &domain.MovementResult{
				ID:            101,
				MovementIDs:   []int64{101},
				DocumentNo:    cmd.DocumentNo,
				SKU:           cmd.SKU,
				LocationID:    cmd.LocationID,
				OperationType: cmd.OperationType,
				Quantity:      decimal.NewFromFloat(20),
				CurrentStock:  decimal.NewFromFloat(35),
				CreatedAt:     now,
			}, nil
		}

		payload := map[string]any{
			"document_no":    "DOC-REC-001",
			"operation_date": now.Format(time.RFC3339),
			"sku":            "OIL-001",
			"location_id":    "loc-1",
			"operation_type": "receipt",
			"quantity":       20.0,
		}
		data, _ := json.Marshal(payload)

		resp, err := http.Post(h.server.URL+"/api/movements", "application/json", bytes.NewReader(data))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		var res dto.MovementResponse
		err = json.NewDecoder(resp.Body).Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, int64(101), res.ID)
		assert.True(t, res.CurrentStock.Equal(decimal.NewFromFloat(35)))
	})

	t.Run("409 Conflict на повторный document_no", func(t *testing.T) {
		h.movementSvc.processFunc = func(ctx context.Context, cmd service.CreateMovementCommand) (*domain.MovementResult, error) {
			return nil, domain.ErrDuplicateDocument
		}

		payload := map[string]any{
			"document_no":    "DOC-DUP-999",
			"operation_date": now.Format(time.RFC3339),
			"sku":            "OIL-001",
			"location":       "loc-1",
			"operation_type": "receipt",
			"quantity":       5.0,
		}
		data, _ := json.Marshal(payload)

		resp, err := http.Post(h.server.URL+"/api/movements", "application/json", bytes.NewReader(data))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		var errResp dto.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errResp)
		require.NoError(t, err)
		assert.Equal(t, "document already processed", errResp.Error)
	})

	t.Run("422 Unprocessable Entity на дату из будущего (> 1 мин)", func(t *testing.T) {
		future := time.Now().UTC().Add(5 * time.Minute)
		payload := map[string]any{
			"document_no":    "DOC-FUTURE",
			"operation_date": future.Format(time.RFC3339),
			"sku":            "OIL-001",
			"location":       "loc-1",
			"operation_type": "consume",
			"quantity":       1.0,
		}
		data, _ := json.Marshal(payload)

		resp, err := http.Post(h.server.URL+"/api/movements", "application/json", bytes.NewReader(data))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

// 7. Тестирование проверки работоспособности GET /health
func TestHealthCheck(t *testing.T) {
	h := setupTestHarness(t)

	t.Run("200 OK при доступной базе данных", func(t *testing.T) {
		h.pinger.err = nil
		resp, err := http.Get(h.server.URL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var body dto.HealthResponse
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "ok", body.Status)
		assert.Equal(t, "connected", body.Database)
	})

	t.Run("503 Service Unavailable при недоступности БД", func(t *testing.T) {
		h.pinger.err = errors.New("database down")
		resp, err := http.Get(h.server.URL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		var body dto.HealthResponse
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "degraded", body.Status)
	})
}

// 8. Тестирование CORS и Panic Recovery
func TestCORS_And_Recovery(t *testing.T) {
	h := setupTestHarness(t)

	t.Run("Preflight OPTIONS запрос возвращает 200 и заголовки CORS", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodOptions, h.server.URL+"/api/movements", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	})

	t.Run("Panic в обработчике перехватывается с ответом 500 JSON", func(t *testing.T) {
		h.stockSvc.summaryFunc = func(ctx context.Context, locationID *string) ([]domain.StockItem, error) {
			panic("simulated fatal nil pointer inside handler")
		}

		resp, err := http.Get(h.server.URL + "/api/stock")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		var errResp dto.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errResp)
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, errResp.Code)
		assert.Contains(t, errResp.Error, "simulated fatal nil pointer")
	})
}

// 9. Тестирование GET /api/stock и GET /api/stock/{sku}
func TestStock_Endpoints(t *testing.T) {
	h := setupTestHarness(t)

	t.Run("GET /api/stock сводка остатков с фильтром по локации", func(t *testing.T) {
		h.stockSvc.summaryFunc = func(ctx context.Context, locationID *string) ([]domain.StockItem, error) {
			assert.Equal(t, "loc-1", *locationID)
			expDate := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
			days := decimal.NewFromFloat(20.0)
			return []domain.StockItem{
				{
					SKU:                 "OIL-001",
					Name:                "Масло лаванда 500мл",
					LocationID:          "loc-1",
					Unit:                "шт",
					CurrentStock:        decimal.NewFromFloat(10.0),
					AvgDailyConsumption: decimal.NewFromFloat(0.5),
					StockDays:           &days,
					NearestExpiryDate:   &expDate,
				},
			}, nil
		}

		resp, err := http.Get(h.server.URL + "/api/stock?location=loc-1")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var items []dto.StockItemResponse
		err = json.NewDecoder(resp.Body).Decode(&items)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "OIL-001", items[0].SKU)
		assert.True(t, items[0].CurrentStock.Equal(decimal.NewFromFloat(10.0)))
	})

	t.Run("GET /api/stock/{sku} детальная информация по партиям", func(t *testing.T) {
		h.stockSvc.detailFunc = func(ctx context.Context, sku string) (*domain.StockDetail, error) {
			assert.Equal(t, "OIL-001", sku)
			return &domain.StockDetail{
				SKU:  "OIL-001",
				Name: "Масло лаванда 500мл",
				Unit: "шт",
				Locations: []domain.LocationStockBreakdown{
					{
						LocationID:   "loc-1",
						LocationName: "Spa Grand",
						TotalStock:   decimal.NewFromFloat(10.0),
						Batches: []domain.BatchStock{
							{
								BatchID:       "B-100",
								Quantity:      decimal.NewFromFloat(10.0),
								PurchasePrice: decimal.NewFromFloat(1500.0),
							},
						},
					},
				},
			}, nil
		}

		resp, err := http.Get(h.server.URL + "/api/stock/OIL-001")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var detail dto.StockDetailResponse
		err = json.NewDecoder(resp.Body).Decode(&detail)
		require.NoError(t, err)
		assert.Equal(t, "OIL-001", detail.SKU)
		require.Len(t, detail.Locations, 1)
		assert.Equal(t, "B-100", detail.Locations[0].Batches[0].Batch)
	})

	t.Run("GET /api/stock/{sku} 404 на неизвестный артикул", func(t *testing.T) {
		h.stockSvc.detailFunc = func(ctx context.Context, sku string) (*domain.StockDetail, error) {
			return nil, domain.ErrNotFound
		}

		resp, err := http.Get(h.server.URL + "/api/stock/UNKNOWN")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// 10. Тестирование GET /api/alerts и справочников
func TestAlerts_And_References(t *testing.T) {
	h := setupTestHarness(t)

	t.Run("GET /api/alerts возвращает предупреждения", func(t *testing.T) {
		h.alertSvc.alertsFunc = func(ctx context.Context, locationID *string) ([]domain.Alert, error) {
			return []domain.Alert{
				{
					Level:      domain.AlertLevelCritical,
					Type:       domain.AlertTypeStockoutRisk,
					SKU:        "OIL-001",
					Name:       "Масло лаванда",
					LocationID: "loc-1",
					Message:    "Риск дефицита",
				},
			}, nil
		}

		resp, err := http.Get(h.server.URL + "/api/alerts")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var alerts []dto.AlertResponse
		err = json.NewDecoder(resp.Body).Decode(&alerts)
		require.NoError(t, err)
		require.Len(t, alerts, 1)
		assert.Equal(t, domain.AlertLevelCritical, alerts[0].Level)
		assert.Equal(t, domain.AlertTypeStockoutRisk, alerts[0].Type)
	})

	t.Run("GET /api/locations, /api/suppliers, /api/products", func(t *testing.T) {
		h.productRepo.locationsFunc = func(ctx context.Context) ([]domain.Location, error) {
			return []domain.Location{{ID: "loc-1", Name: "Spa Grand"}}, nil
		}
		h.productRepo.suppliersFunc = func(ctx context.Context) ([]domain.Supplier, error) {
			return []domain.Supplier{{ID: "sup-1", Name: "АромаЛюкс"}}, nil
		}
		h.productRepo.productsFunc = func(ctx context.Context) ([]domain.Product, error) {
			return []domain.Product{{SKU: "OIL-001", Name: "Масло"}}, nil
		}

		respLoc, err := http.Get(h.server.URL + "/api/locations")
		require.NoError(t, err)
		respLoc.Body.Close()
		assert.Equal(t, http.StatusOK, respLoc.StatusCode)

		respSup, err := http.Get(h.server.URL + "/api/suppliers")
		require.NoError(t, err)
		respSup.Body.Close()
		assert.Equal(t, http.StatusOK, respSup.StatusCode)

		respProd, err := http.Get(h.server.URL + "/api/products")
		require.NoError(t, err)
		respProd.Body.Close()
		assert.Equal(t, http.StatusOK, respProd.StatusCode)
	})
}
