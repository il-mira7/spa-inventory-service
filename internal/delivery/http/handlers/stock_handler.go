package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/dto"
	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/il-mira7/spa-inventory-service/internal/service"
)

// StockHandler обрабатывает запросы к остаткам товаров
type StockHandler struct {
	stockService service.StockService
}

// NewStockHandler создает экземпляр обработчика остатков
func NewStockHandler(stockService service.StockService) *StockHandler {
	return &StockHandler{stockService: stockService}
}

// Summary обрабатывает GET /api/stock?location=
func (h *StockHandler) Summary(w http.ResponseWriter, r *http.Request) {
	var locParam *string
	loc := r.URL.Query().Get("location")
	if loc == "" {
		loc = r.URL.Query().Get("location_id")
	}
	if loc != "" {
		locParam = &loc
	}

	items, err := h.stockService.GetStockSummary(r.Context(), locParam)
	if err != nil {
		Error(w, err)
		return
	}

	resp := make([]dto.StockItemResponse, 0, len(items))
	for _, it := range items {
		var nearestStr *string
		if it.NearestExpiryDate != nil {
			s := it.NearestExpiryDate.Format("2006-01-02")
			nearestStr = &s
		}

		resp = append(resp, dto.StockItemResponse{
			SKU:                 it.SKU,
			Name:                it.Name,
			Location:            it.LocationID,
			Unit:                it.Unit,
			CurrentStock:        it.CurrentStock,
			AvgDailyConsumption: it.AvgDailyConsumption,
			StockDays:           it.StockDays,
			NearestExpiryDate:   nearestStr,
		})
	}

	JSON(w, http.StatusOK, resp)
}

// Detail обрабатывает GET /api/stock/{sku}
func (h *StockHandler) Detail(w http.ResponseWriter, r *http.Request) {
	sku := chi.URLParam(r, "sku")
	if sku == "" {
		Error(w, domain.ErrNotFound)
		return
	}

	detail, err := h.stockService.GetStockDetail(r.Context(), sku)
	if err != nil {
		Error(w, err)
		return
	}

	locations := make([]dto.LocationStockBreakdownResponse, 0, len(detail.Locations))
	for _, l := range detail.Locations {
		batches := make([]dto.BatchStockResponse, 0, len(l.Batches))
		for _, b := range l.Batches {
			var expStr *string
			if b.ExpiryDate != nil {
				s := b.ExpiryDate.Format(time.DateOnly)
				expStr = &s
			}

			price := b.PurchasePrice
			batches = append(batches, dto.BatchStockResponse{
				Batch:         b.BatchID,
				Quantity:      b.Quantity,
				ExpiryDate:    expStr,
				PurchasePrice: price,
			})
		}

		locations = append(locations, dto.LocationStockBreakdownResponse{
			LocationID:   l.LocationID,
			LocationName: l.LocationName,
			TotalStock:   l.TotalStock,
			Batches:      batches,
		})
	}

	JSON(w, http.StatusOK, dto.StockDetailResponse{
		SKU:       detail.SKU,
		Name:      detail.Name,
		Unit:      detail.Unit,
		Locations: locations,
	})
}
