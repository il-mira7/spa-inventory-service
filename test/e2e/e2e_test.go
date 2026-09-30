package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getBaseURL() string {
	url := os.Getenv("TEST_BASE_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	return url
}

func checkServiceAvailable(t *testing.T, baseURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Logf("Служба недоступна по адресу %s: %v. Пропуск E2E тестов.", baseURL, err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// TestE2E_FullSuite запускает сквозное тестирование полного контура приложения
func TestE2E_FullSuite(t *testing.T) {
	baseURL := getBaseURL()

	if !checkServiceAvailable(t, baseURL) {
		t.Skipf("Сервис не запущен на %s. Запустите 'make docker-up' перед прогоном E2E тестов.", baseURL)
		return
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Не переходить по редиректам автоматически
		},
	}

	// -------------------------------------------------------------------------
	// 1. Healthcheck
	// -------------------------------------------------------------------------
	t.Run("1. GET /health возвращает 200 OK и статус базы connected", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		assert.Equal(t, "ok", body["status"])
		assert.Equal(t, "connected", body["database"])
		assert.Equal(t, "1.0.0", body["version"])
	})

	// -------------------------------------------------------------------------
	// 2. Swagger & Docs Redirect
	// -------------------------------------------------------------------------
	t.Run("2. GET /docs редиректит на /swagger/, а Swagger UI отдает HTML", func(t *testing.T) {
		// Проверка 301 редиректа
		resp, err := client.Get(baseURL + "/docs")
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
		assert.Equal(t, "/swagger/", resp.Header.Get("Location"))

		// Проверка доступности Swagger UI
		respUI, err := client.Get(baseURL + "/swagger/")
		require.NoError(t, err)
		defer respUI.Body.Close()

		assert.Equal(t, http.StatusOK, respUI.StatusCode)
		htmlBody, err := io.ReadAll(respUI.Body)
		require.NoError(t, err)
		assert.Contains(t, string(htmlBody), "swagger-ui")

		// Проверка спецификации openapi.yaml
		respSpec, err := client.Get(baseURL + "/swagger/openapi.yaml")
		require.NoError(t, err)
		defer respSpec.Body.Close()

		assert.Equal(t, http.StatusOK, respSpec.StatusCode)
		specBody, err := io.ReadAll(respSpec.Body)
		require.NoError(t, err)
		assert.Contains(t, string(specBody), "openapi: 3.0.3")
	})

	// -------------------------------------------------------------------------
	// 3. Справочники (References)
	// -------------------------------------------------------------------------
	t.Run("3. GET /api/locations, /api/suppliers, /api/products отдают сидовые данные", func(t *testing.T) {
		// Locations
		resp, err := client.Get(baseURL + "/api/locations")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var locs []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&locs))
		assert.GreaterOrEqual(t, len(locs), 2)

		// Suppliers
		respSup, err := client.Get(baseURL + "/api/suppliers")
		require.NoError(t, err)
		defer respSup.Body.Close()
		assert.Equal(t, http.StatusOK, respSup.StatusCode)
		var sups []map[string]interface{}
		require.NoError(t, json.NewDecoder(respSup.Body).Decode(&sups))
		assert.GreaterOrEqual(t, len(sups), 3)

		// Products
		respProd, err := client.Get(baseURL + "/api/products")
		require.NoError(t, err)
		defer respProd.Body.Close()
		assert.Equal(t, http.StatusOK, respProd.StatusCode)
		var prods []map[string]interface{}
		require.NoError(t, json.NewDecoder(respProd.Body).Decode(&prods))
		assert.GreaterOrEqual(t, len(prods), 5)
	})

	// -------------------------------------------------------------------------
	// 4. Остатки (Stock)
	// -------------------------------------------------------------------------
	t.Run("4. GET /api/stock и /api/stock/{sku} отдают актуальные данные по остаткам", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/api/stock")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var items []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&items))
		assert.NotEmpty(t, items)

		// Проверка конкретного товара
		respSku, err := client.Get(baseURL + "/api/stock/OIL-001")
		require.NoError(t, err)
		defer respSku.Body.Close()
		assert.Equal(t, http.StatusOK, respSku.StatusCode)

		var stockDetail map[string]interface{}
		require.NoError(t, json.NewDecoder(respSku.Body).Decode(&stockDetail))
		assert.Equal(t, "OIL-001", stockDetail["sku"])

		locations, ok := stockDetail["locations"].([]interface{})
		require.True(t, ok)
		require.NotEmpty(t, locations)

		firstLoc := locations[0].(map[string]interface{})
		batches, ok := firstLoc["batches"].([]interface{})
		require.True(t, ok)
		assert.NotEmpty(t, batches)

		// 404 на несуществующий SKU
		resp404, err := client.Get(baseURL + "/api/stock/NON-EXISTENT-SKU")
		require.NoError(t, err)
		resp404.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp404.StatusCode)
	})

	// -------------------------------------------------------------------------
	// 5. Движения товаров: Оприходование и Списание по FEFO
	// -------------------------------------------------------------------------
	uniqueSuffix := time.Now().UnixNano()
	docReceipt := fmt.Sprintf("E2E-REC-%d", uniqueSuffix)
	docConsume := fmt.Sprintf("E2E-CON-%d", uniqueSuffix)
	var stockAfterReceipt float64

	t.Run("5.1. POST /api/movements (receipt) создает партию и возвращает пересчитанный current_stock", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"document_no":    docReceipt,
			"operation_type": "receipt",
			"sku":            "OIL-001",
			"location_id":    "MS-01",
			"batch_no":       fmt.Sprintf("BATCH-E2E-%d", uniqueSuffix),
			"expiry_date":    time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
			"quantity":       25.0,
			"unit_price":     1500.0,
			"operation_date": time.Now().Format(time.RFC3339),
		}

		payload, _ := json.Marshal(reqBody)
		resp, err := client.Post(baseURL+"/api/movements", "application/json", bytes.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res["id"])
		assert.NotEmpty(t, res["current_stock"])

		stockStr := fmt.Sprintf("%v", res["current_stock"])
		var errParse error
		stockAfterReceipt, errParse = strconv.ParseFloat(stockStr, 64)
		require.NoError(t, errParse)
		assert.GreaterOrEqual(t, stockAfterReceipt, 25.0)
	})

	t.Run("5.2. POST /api/movements (consume) списывает по FEFO и возвращает уменьшенный current_stock", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"document_no":    docConsume,
			"operation_type": "consume",
			"sku":            "OIL-001",
			"location_id":    "MS-01",
			"quantity":       10.0,
			"operation_date": time.Now().Format(time.RFC3339),
		}

		payload, _ := json.Marshal(reqBody)
		resp, err := client.Post(baseURL+"/api/movements", "application/json", bytes.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res["id"])

		stockStr := fmt.Sprintf("%v", res["current_stock"])
		stockAfterConsume, errParse := strconv.ParseFloat(stockStr, 64)
		require.NoError(t, errParse)
		assert.InDelta(t, stockAfterReceipt-10.0, stockAfterConsume, 0.001)
	})

	// -------------------------------------------------------------------------
	// 6. Валидации и обработка ошибок
	// -------------------------------------------------------------------------
	t.Run("6.1. Дата из будущего (> 1 мин) возвращает HTTP 422", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"document_no":    fmt.Sprintf("ERR-FUT-%d", uniqueSuffix),
			"operation_type": "receipt",
			"sku":            "OIL-001",
			"location_id":    "MS-01",
			"batch_no":       "B-ERR",
			"expiry_date":    "2028-01-01",
			"quantity":       1.0,
			"unit_price":     100.0,
			"operation_date": time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		}

		payload, _ := json.Marshal(reqBody)
		resp, err := client.Post(baseURL+"/api/movements", "application/json", bytes.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("6.2. Повторный document_no возвращает HTTP 409 Conflict", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"document_no":    docReceipt, // Уже использован в тесте 5.1
			"operation_type": "receipt",
			"sku":            "OIL-001",
			"location_id":    "MS-01",
			"batch_no":       "B-DUP",
			"expiry_date":    "2028-01-01",
			"quantity":       5.0,
			"unit_price":     100.0,
			"operation_date": time.Now().Format(time.RFC3339),
		}

		payload, _ := json.Marshal(reqBody)
		resp, err := client.Post(baseURL+"/api/movements", "application/json", bytes.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("6.3. Нехватка остатка возвращает HTTP 422 с available_stock и requested", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"document_no":    fmt.Sprintf("ERR-EXC-%d", uniqueSuffix),
			"operation_type": "consume",
			"sku":            "OIL-001",
			"location_id":    "MS-01",
			"quantity":       999999.0, // Заведомо больше остатка
			"operation_date": time.Now().Format(time.RFC3339),
		}

		payload, _ := json.Marshal(reqBody)
		resp, err := client.Post(baseURL+"/api/movements", "application/json", bytes.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

		var errRes map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errRes))
		assert.Equal(t, "OIL-001", errRes["sku"])
		assert.Equal(t, float64(999999), errRes["requested"])
		assert.NotNil(t, errRes["available_stock"])
	})

	// -------------------------------------------------------------------------
	// 7. Прогнозирование (Forecast)
	// -------------------------------------------------------------------------
	t.Run("7. POST /api/forecast рассчитывает прогноз и блок explanation", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"sku":          "OIL-001",
			"location_id":  "MS-01",
			"horizon_days": 30,
		}

		payload, _ := json.Marshal(reqBody)
		resp, err := client.Post(baseURL+"/api/forecast", "application/json", bytes.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var fc map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&fc))

		assert.Equal(t, "OIL-001", fc["sku"])
		assert.Equal(t, "MS-01", fc["location"])
		assert.NotNil(t, fc["recommended_purchase_qty"])
		assert.NotNil(t, fc["avg_daily_consumption"])
		assert.NotEmpty(t, fc["explanation"])
	})

	// -------------------------------------------------------------------------
	// 8. Предупреждения (Alerts)
	// -------------------------------------------------------------------------
	t.Run("8. GET /api/alerts возвращает список предупреждений системы", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/api/alerts")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var alerts []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&alerts))
		// В сидах присутствуют товары с истекающим сроком или низким остатком
		assert.NotEmpty(t, alerts)
	})
}
