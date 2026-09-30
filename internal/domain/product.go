package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Product представляет товар номенклатурного справочника
type Product struct {
	SKU               string          `json:"sku"`
	Name              string          `json:"name"`
	Category          string          `json:"category"`
	Unit              string          `json:"unit"`
	PackageSize       decimal.Decimal `json:"package_size"`
	MinOrderQty       decimal.Decimal `json:"min_order_qty"`
	LeadTimeDays      int             `json:"lead_time_days"`
	DefaultSupplierID *string         `json:"default_supplier_id,omitempty"`
	LastPurchasePrice decimal.Decimal `json:"last_purchase_price"`
	IsActive          bool            `json:"is_active"`
	CreatedAt         time.Time       `json:"created_at"`
}

// Location представляет объект / филиал / спа-комплекс
type Location struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Supplier представляет поставщика товаров
type Supplier struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	ContactPerson string    `json:"contact_person,omitempty"`
	Phone         string    `json:"phone,omitempty"`
	Email         string    `json:"email,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// OpenOrder представляет открытый заказ в пути (ожидаемая поставка)
type OpenOrder struct {
	ID                   string          `json:"id"`
	SKU                  string          `json:"sku"`
	LocationID           string          `json:"location_id"`
	SupplierID           string          `json:"supplier_id"`
	Quantity             decimal.Decimal `json:"quantity"`
	OrderDate            time.Time       `json:"order_date"`
	ExpectedDeliveryDate time.Time       `json:"expected_delivery_date"`
	Status               string          `json:"status"` // in_transit, delivered, cancelled
	CreatedAt            time.Time       `json:"created_at"`
}
