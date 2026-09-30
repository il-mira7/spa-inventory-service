package dto

// ErrorResponse представляет унифицированный ответ с ошибкой
type ErrorResponse struct {
	Error          string   `json:"error"`
	Code           int      `json:"code,omitempty"`
	SKU            string   `json:"sku,omitempty"`
	AvailableStock *float64 `json:"available_stock,omitempty"`
	Available      *float64 `json:"available,omitempty"`
	Requested      *float64 `json:"requested,omitempty"`
}
