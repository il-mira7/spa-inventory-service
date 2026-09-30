package handlers

import (
	"net/http"

	"github.com/il-mira7/spa-inventory-service/internal/repository/postgres"
)

// ReferenceHandler обслуживает запросы к справочникам системы
type ReferenceHandler struct {
	productRepo postgres.ProductRepository
}

// NewReferenceHandler создает экземпляр обработчика справочников
func NewReferenceHandler(productRepo postgres.ProductRepository) *ReferenceHandler {
	return &ReferenceHandler{productRepo: productRepo}
}

// GetLocations обрабатывает GET /api/locations
func (h *ReferenceHandler) GetLocations(w http.ResponseWriter, r *http.Request) {
	locations, err := h.productRepo.GetLocations(r.Context())
	if err != nil {
		Error(w, err)
		return
	}
	JSON(w, http.StatusOK, locations)
}

// GetSuppliers обрабатывает GET /api/suppliers
func (h *ReferenceHandler) GetSuppliers(w http.ResponseWriter, r *http.Request) {
	suppliers, err := h.productRepo.GetSuppliers(r.Context())
	if err != nil {
		Error(w, err)
		return
	}
	JSON(w, http.StatusOK, suppliers)
}

// GetProducts обрабатывает GET /api/products
func (h *ReferenceHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.productRepo.GetAllActiveProducts(r.Context())
	if err != nil {
		Error(w, err)
		return
	}
	JSON(w, http.StatusOK, products)
}
