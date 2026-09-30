package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Batch представляет партию поступления товара (производственная серия)
type Batch struct {
	ID            string          `json:"id"`
	SKU           string          `json:"sku"`
	ExpiryDate    *time.Time      `json:"expiry_date,omitempty"`
	PurchasePrice decimal.Decimal `json:"purchase_price"`
	InvoiceNo     string          `json:"invoice_no,omitempty"`
	ReceivedDate  time.Time       `json:"received_date"`
	CreatedAt     time.Time       `json:"created_at"`
}

// IsExpired проверяет, истек ли срок годности партии на указанную дату
func (b Batch) IsExpired(asOf time.Time) bool {
	if b.ExpiryDate == nil {
		return false
	}
	// Если срок годности меньше или равен текущей дате - партия просрочена
	return !b.ExpiryDate.After(asOf)
}

// BatchStock представляет вычисленный остаток конкретной партии на определенном складе
type BatchStock struct {
	BatchID       string          `json:"batch"`
	SKU           string          `json:"sku"`
	LocationID    string          `json:"location"`
	Quantity      decimal.Decimal `json:"quantity"`
	ExpiryDate    *time.Time      `json:"expiry_date,omitempty"`
	PurchasePrice decimal.Decimal `json:"purchase_price"`
	InvoiceNo     string          `json:"invoice_no,omitempty"`
	ReceivedDate  time.Time       `json:"received_date"`
}

// IsExpired проверяет, истек ли срок годности партии
func (bs BatchStock) IsExpired(asOf time.Time) bool {
	if bs.ExpiryDate == nil {
		return false
	}
	return !bs.ExpiryDate.After(asOf)
}
