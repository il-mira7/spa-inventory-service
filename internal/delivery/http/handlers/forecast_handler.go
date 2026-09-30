package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/dto"
	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/il-mira7/spa-inventory-service/internal/service"
	"github.com/shopspring/decimal"
)

// ForecastHandler обрабатывает запросы к модулю прогнозирования закупок
type ForecastHandler struct {
	forecastService service.ForecastService
}

// NewForecastHandler создает новый экземпляр обработчика прогнозов
func NewForecastHandler(forecastService service.ForecastService) *ForecastHandler {
	return &ForecastHandler{forecastService: forecastService}
}

// Calculate обрабатывает POST /api/forecast
func (h *ForecastHandler) Calculate(w http.ResponseWriter, r *http.Request) {
	var req dto.ForecastRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, domain.ErrInvalidOperation)
		return
	}

	if err := req.Validate(); err != nil {
		Error(w, err)
		return
	}

	from, to, err := req.ParsePeriod()
	if err != nil {
		Error(w, err)
		return
	}

	asOf, err := req.ParseAsOfDate()
	if err != nil {
		Error(w, err)
		return
	}

	unitPrice := decimal.Zero
	if req.UnitPrice != nil {
		unitPrice = decimal.NewFromFloat(*req.UnitPrice)
	}

	safetyDays := 14
	if req.SafetyStockDays != nil && *req.SafetyStockDays > 0 {
		safetyDays = *req.SafetyStockDays
	}

	loc := req.LocationID
	if loc == "" {
		loc = req.Location
	}

	svcReq := service.ForecastRequest{
		SKU:             req.SKU,
		LocationID:      loc,
		PeriodFrom:      from,
		PeriodTo:        to,
		SafetyStockDays: safetyDays,
		UnitPrice:       unitPrice,
		AsOfDate:        asOf,
	}

	res, err := h.forecastService.CalculateForecast(r.Context(), svcReq)
	if err != nil {
		Error(w, err)
		return
	}

	JSON(w, http.StatusOK, dto.ForecastResponseDTO{
		SKU:                    res.SKU,
		Name:                   res.Name,
		Unit:                   res.Unit,
		Location:               res.Location,
		Period:                 res.Period,
		AvgDailyConsumption:    res.AvgDailyConsumption,
		ForecastDemand:         res.ForecastDemand,
		CurrentStock:           res.CurrentStock,
		IncomingQty:            res.IncomingQty,
		SafetyStock:            res.SafetyStock,
		ReorderPoint:           res.ReorderPoint,
		RecommendedPurchaseQty: res.RecommendedPurchaseQty,
		UnitPrice:              res.UnitPrice,
		EstimatedCost:          res.EstimatedCost,
		RecommendedOrderDate:   res.RecommendedOrderDate,
		StockoutDate:           res.StockoutDate,
		Explanation:            res.Explanation,
		Warnings:               res.Warnings,
	})
}
