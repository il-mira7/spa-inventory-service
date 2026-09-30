package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/dto"
	"github.com/il-mira7/spa-inventory-service/internal/domain"
)

// JSON сериализует данные в JSON с установкой заголовков и статус-кода
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// Error преобразует доменные ошибки в стандартизированные HTTP статус-коды
func Error(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	// 1. Ошибка нехватки остатка (422 Unprocessable Entity с расширенным телом: available_stock, requested, sku)
	if stockErr, ok := domain.IsInsufficientStock(err); ok {
		availFloat := stockErr.AvailableStock.InexactFloat64()
		reqFloat := stockErr.Requested.InexactFloat64()
		JSON(w, http.StatusUnprocessableEntity, dto.ErrorResponse{
			Error:          stockErr.Error(),
			Code:           http.StatusUnprocessableEntity,
			SKU:            stockErr.SKU,
			AvailableStock: &availFloat,
			Available:      &availFloat,
			Requested:      &reqFloat,
		})
		return
	}

	// 2. Некорректный запрос / синтаксис JSON (400 Bad Request)
	if errors.Is(err, domain.ErrBadRequest) {
		JSON(w, http.StatusBadRequest, dto.ErrorResponse{
			Error: err.Error(),
			Code:  http.StatusBadRequest,
		})
		return
	}

	// 3. Ресурс не найден (404 Not Found)
	if errors.Is(err, domain.ErrNotFound) {
		JSON(w, http.StatusNotFound, dto.ErrorResponse{
			Error: err.Error(),
			Code:  http.StatusNotFound,
		})
		return
	}

	// 3. Дубликат документа (409 Conflict)
	if errors.Is(err, domain.ErrDuplicateDocument) {
		JSON(w, http.StatusConflict, dto.ErrorResponse{
			Error: "document already processed",
			Code:  http.StatusConflict,
		})
		return
	}

	// 4. Ошибки бизнес-валидации (422 Unprocessable Entity)
	if errors.Is(err, domain.ErrFutureDate) ||
		errors.Is(err, domain.ErrInvalidQuantity) ||
		errors.Is(err, domain.ErrInvalidOperation) ||
		errors.Is(err, domain.ErrInvalidDocumentNo) ||
		errors.Is(err, domain.ErrBatchExpired) {
		JSON(w, http.StatusUnprocessableEntity, dto.ErrorResponse{
			Error: err.Error(),
			Code:  http.StatusUnprocessableEntity,
		})
		return
	}

	// 5. Внутренняя ошибка сервера (500 Internal Server Error)
	JSON(w, http.StatusInternalServerError, dto.ErrorResponse{
		Error: err.Error(),
		Code:  http.StatusInternalServerError,
	})
}
