package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/dto"
	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/il-mira7/spa-inventory-service/internal/repository/postgres"
	"github.com/il-mira7/spa-inventory-service/internal/service"
	"github.com/shopspring/decimal"
)

// MovementHandler обрабатывает запросы к журналу складских проводок
type MovementHandler struct {
	movementService service.MovementService
}

// NewMovementHandler создает экземпляр обработчика движений
func NewMovementHandler(movementService service.MovementService) *MovementHandler {
	return &MovementHandler{movementService: movementService}
}

// Create обрабатывает POST /api/movements
func (h *MovementHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateMovementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, domain.ErrInvalidOperation)
		return
	}

	if err := req.Validate(); err != nil {
		Error(w, err)
		return
	}

	opDate, err := req.ParseOperationDate()
	if err != nil {
		Error(w, err)
		return
	}

	expiryDate, err := req.ParseExpiryDate()
	if err != nil {
		Error(w, err)
		return
	}

	price := decimal.Zero
	if req.PurchasePrice != nil {
		price = decimal.NewFromFloat(*req.PurchasePrice)
	}

	loc := req.LocationID
	if loc == "" {
		loc = req.Location
	}

	cmd := service.CreateMovementCommand{
		DocumentNo:    req.DocumentNo,
		OperationDate: opDate,
		SKU:           req.SKU,
		LocationID:    loc,
		OperationType: domain.OperationType(req.OperationType),
		Quantity:      decimal.NewFromFloat(req.Quantity),
		BatchID:       req.BatchID,
		ExpiryDate:    expiryDate,
		PurchasePrice: price,
		InvoiceNo:     req.InvoiceNo,
		Note:          req.Note,
	}

	res, err := h.movementService.ProcessMovement(r.Context(), cmd)
	if err != nil {
		Error(w, err)
		return
	}

	JSON(w, http.StatusCreated, dto.MovementResponse{
		ID:            res.ID,
		MovementIDs:   res.MovementIDs,
		DocumentNo:    res.DocumentNo,
		SKU:           res.SKU,
		Location:      res.LocationID,
		OperationType: string(res.OperationType),
		Quantity:      res.Quantity,
		CurrentStock:  res.CurrentStock,
		CreatedAt:     res.CreatedAt.Format(time.RFC3339),
	})
}

// List обрабатывает GET /api/movements с фильтрами и пагинацией
func (h *MovementHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := postgres.MovementFilter{}

	// 1. Фильтр по артикулу (sku)
	if sku := q.Get("sku"); sku != "" {
		filter.SKU = &sku
	}

	// 2. Фильтр по локации (location / location_id)
	loc := q.Get("location")
	if loc == "" {
		loc = q.Get("location_id")
	}
	if loc != "" {
		filter.LocationID = &loc
	}

	// 3. Фильтр по типу операции (operation_type / type)
	opType := q.Get("operation_type")
	if opType == "" {
		opType = q.Get("type")
	}
	if opType != "" {
		ot := domain.OperationType(opType)
		if ot.IsValid() {
			filter.OperationType = &ot
		}
	}

	// 4. Фильтр начальной даты (from_date / from)
	fromStr := q.Get("from_date")
	if fromStr == "" {
		fromStr = q.Get("from")
	}
	if fromStr != "" {
		if t, err := parseDateFilter(fromStr); err == nil {
			filter.FromDate = &t
		}
	}

	// 5. Фильтр конечной даты (to_date / to)
	toStr := q.Get("to_date")
	if toStr == "" {
		toStr = q.Get("to")
	}
	if toStr != "" {
		if t, err := parseDateFilter(toStr); err == nil {
			filter.ToDate = &t
		}
	}

	// 6. Пагинация limit и offset
	limit := 50
	if l := q.Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	offset := 0
	if o := q.Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	items, total, err := h.movementService.GetMovements(r.Context(), filter, limit, offset)
	if err != nil {
		Error(w, err)
		return
	}

	respItems := make([]dto.MovementListItemResponse, 0, len(items))
	for _, m := range items {
		respItems = append(respItems, dto.MovementListItemResponse{
			ID:            m.ID,
			DocumentNo:    m.DocumentNo,
			OperationDate: m.OperationDate.Format(time.RFC3339),
			SKU:           m.SKU,
			Location:      m.LocationID,
			Type:          string(m.OperationType),
			Quantity:      m.Quantity,
			Batch:         m.BatchID,
			Note:          m.Note,
			CreatedAt:     m.CreatedAt.Format(time.RFC3339),
		})
	}

	JSON(w, http.StatusOK, dto.MovementsListResponse{
		Items:  respItems,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func parseDateFilter(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		time.RFC3339,
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, strconv.ErrSyntax
}
