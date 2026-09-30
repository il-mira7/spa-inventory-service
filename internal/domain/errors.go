package domain

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// Базовые ошибки предметной области
var (
	ErrNotFound          = errors.New("resource not found")
	ErrDuplicateDocument = errors.New("document already processed")
	ErrFutureDate        = errors.New("operation date cannot be in the future")
	ErrInvalidQuantity   = errors.New("quantity must be strictly greater than zero")
	ErrBatchExpired      = errors.New("batch is expired and cannot be consumed")
	ErrInvalidOperation  = errors.New("invalid operation type")
)

// InsufficientStockError сигнализирует о нехватке остатка для списания (HTTP 422).
type InsufficientStockError struct {
	SKU            string
	LocationID     string
	Requested      decimal.Decimal
	AvailableStock decimal.Decimal
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("insufficient stock for SKU %s at %s: requested %s, available %s",
		e.SKU, e.LocationID, e.Requested.String(), e.AvailableStock.String())
}

// NewInsufficientStockError создает ошибку нехватки остатка.
func NewInsufficientStockError(sku, locationID string, requested, available decimal.Decimal) error {
	return &InsufficientStockError{
		SKU:            sku,
		LocationID:     locationID,
		Requested:      requested,
		AvailableStock: available,
	}
}

// IsInsufficientStock проверяет, является ли ошибка ошибкой нехватки остатка.
func IsInsufficientStock(err error) (*InsufficientStockError, bool) {
	var target *InsufficientStockError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
