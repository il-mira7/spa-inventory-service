package dto

import (
	"fmt"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/shopspring/decimal"
)

// CreateMovementRequest DTO для создания складской проводки через POST /api/movements
type CreateMovementRequest struct {
	DocumentNo    string   `json:"document_no"`
	OperationDate string   `json:"operation_date"`
	SKU           string   `json:"sku"`
	Location      string   `json:"location"`
	LocationID    string   `json:"location_id"`
	OperationType string   `json:"operation_type"`
	Quantity      float64  `json:"quantity"`
	BatchID       string   `json:"batch_id,omitempty"`
	ExpiryDate    *string  `json:"expiry_date,omitempty"`
	PurchasePrice *float64 `json:"purchase_price,omitempty"`
	InvoiceNo     string   `json:"invoice_no,omitempty"`
	Note          string   `json:"note,omitempty"`
}

// Validate проверяет корректность входных данных
func (r *CreateMovementRequest) Validate() error {
	if r.DocumentNo == "" {
		return domain.ErrInvalidDocumentNo
	}
	if r.SKU == "" {
		return fmt.Errorf("sku is required: %w", domain.ErrInvalidOperation)
	}
	if r.LocationID == "" {
		r.LocationID = r.Location
	}
	if r.LocationID == "" {
		return fmt.Errorf("location is required: %w", domain.ErrInvalidOperation)
	}
	opType := domain.OperationType(r.OperationType)
	if !opType.IsValid() {
		return domain.ErrInvalidOperation
	}
	if r.Quantity <= 0 && opType != domain.OperationCorrection {
		return domain.ErrInvalidQuantity
	}
	if (opType == domain.OperationWriteoff || opType == domain.OperationReturn) && r.BatchID == "" {
		return fmt.Errorf("batch_id is required for %s: %w", r.OperationType, domain.ErrInvalidOperation)
	}
	return nil
}

// ParseOperationDate парсит переданную дату операции с валидацией будущего времени
func (r *CreateMovementRequest) ParseOperationDate() (time.Time, error) {
	if r.OperationDate == "" {
		return time.Now(), nil
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, layout := range formats {
		if t, err := time.Parse(layout, r.OperationDate); err == nil {
			if t.After(time.Now().Add(1 * time.Minute)) {
				return time.Time{}, domain.ErrFutureDate
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid operation_date format: %w", domain.ErrInvalidOperation)
}

// ParseExpiryDate парсит срок годности для поступлений
func (r *CreateMovementRequest) ParseExpiryDate() (*time.Time, error) {
	if r.ExpiryDate == nil || *r.ExpiryDate == "" {
		return nil, nil
	}

	formats := []string{
		"2006-01-02",
		time.RFC3339,
	}

	for _, layout := range formats {
		if t, err := time.Parse(layout, *r.ExpiryDate); err == nil {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("invalid expiry_date format: %w", domain.ErrInvalidOperation)
}

// MovementResponse DTO ответа на POST /api/movements
type MovementResponse struct {
	ID            int64           `json:"id"`
	MovementIDs   []int64         `json:"movement_ids,omitempty"`
	DocumentNo    string          `json:"document_no"`
	SKU           string          `json:"sku"`
	Location      string          `json:"location"`
	OperationType string          `json:"operation_type"`
	Quantity      decimal.Decimal `json:"quantity"`
	CurrentStock  decimal.Decimal `json:"current_stock"`
	CreatedAt     string          `json:"created_at"`
}

// MovementListItemResponse DTO элемента списка движений GET /api/movements
type MovementListItemResponse struct {
	ID            int64           `json:"id"`
	DocumentNo    string          `json:"document_no"`
	OperationDate string          `json:"operation_date"`
	SKU           string          `json:"sku"`
	Location      string          `json:"location"`
	Type          string          `json:"type"`
	Quantity      decimal.Decimal `json:"quantity"`
	Batch         string          `json:"batch,omitempty"`
	Note          string          `json:"note,omitempty"`
	CreatedAt     string          `json:"created_at"`
}

// MovementsListResponse DTO ответа списка движений с пагинацией
type MovementsListResponse struct {
	Items  []MovementListItemResponse `json:"items"`
	Total  int64                      `json:"total"`
	Limit  int                        `json:"limit"`
	Offset int                        `json:"offset"`
}
