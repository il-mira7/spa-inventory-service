package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// OperationType задает тип складской операции
type OperationType string

const (
	OperationReceipt    OperationType = "receipt"    // Поступление товара
	OperationConsume    OperationType = "consume"    // Расход на процедуры
	OperationWriteoff   OperationType = "writeoff"   // Списание (брак / просрочка)
	OperationReturn     OperationType = "return"     // Возврат неиспользованного товара
	OperationCorrection OperationType = "correction" // Инвентаризационная корректировка
)

// IsValid проверяет допустимость типа операции
func (t OperationType) IsValid() bool {
	switch t {
	case OperationReceipt, OperationConsume, OperationWriteoff, OperationReturn, OperationCorrection:
		return true
	default:
		return false
	}
}

// Movement представляет единичную запись в журнале движений (Append-Only Ledger)
type Movement struct {
	ID            int64           `json:"id"`
	DocumentNo    string          `json:"document_no"`
	OperationDate time.Time       `json:"operation_date"`
	SKU           string          `json:"sku"`
	LocationID    string          `json:"location"`
	OperationType OperationType   `json:"type"`
	Quantity      decimal.Decimal `json:"quantity"`
	BatchID       string          `json:"batch"`
	Note          string          `json:"note,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// Multiplier возвращает математический знак для расчета остатка:
// receipt: +1, return: +1, consume: -1, writeoff: -1, correction: +1 (или знак самого quantity)
func (m Movement) SignedQuantity() decimal.Decimal {
	switch m.OperationType {
	case OperationReceipt, OperationReturn:
		return m.Quantity.Abs()
	case OperationConsume, OperationWriteoff:
		return m.Quantity.Abs().Neg()
	case OperationCorrection:
		return m.Quantity // при инвентаризации знак передается явно
	default:
		return decimal.Zero
	}
}

// MovementResult представляет результат выполнения складской операции для ответа POST /api/movements
type MovementResult struct {
	ID            int64           `json:"id"`
	MovementIDs   []int64         `json:"movement_ids,omitempty"`
	DocumentNo    string          `json:"document_no"`
	SKU           string          `json:"sku"`
	LocationID    string          `json:"location"`
	OperationType OperationType   `json:"type"`
	Quantity      decimal.Decimal `json:"quantity"`
	CurrentStock  decimal.Decimal `json:"current_stock"`
	CreatedAt     time.Time       `json:"created_at"`
}
