package service

import (
	"context"
	"fmt"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/calc"
	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/il-mira7/spa-inventory-service/internal/repository/postgres"
	"github.com/shopspring/decimal"
)

// ForecastRequest содержит входные параметры для расчета прогноза закупки
type ForecastRequest struct {
	SKU             string          `json:"sku"`
	LocationID      string          `json:"location"`
	PeriodFrom      time.Time       `json:"period_from"`
	PeriodTo        time.Time       `json:"period_to"`
	SafetyStockDays int             `json:"safety_stock_days"`
	UnitPrice       decimal.Decimal `json:"unit_price"`
	AsOfDate        *time.Time      `json:"as_of_date,omitempty"`
}

// ForecastService определяет контракт бизнес-логики прогнозирования потребностей и закупок
type ForecastService interface {
	CalculateForecast(ctx context.Context, req ForecastRequest) (*domain.ForecastResult, error)
}

type forecastService struct {
	productRepo  postgres.ProductRepository
	stockRepo    postgres.StockRepository
	orderRepo    postgres.OrderRepository
	movementRepo postgres.MovementRepository
}

// NewForecastService создает новый экземпляр сервиса прогнозирования
func NewForecastService(
	productRepo postgres.ProductRepository,
	stockRepo postgres.StockRepository,
	orderRepo postgres.OrderRepository,
	movementRepo postgres.MovementRepository,
) ForecastService {
	return &forecastService{
		productRepo:  productRepo,
		stockRepo:    stockRepo,
		orderRepo:    orderRepo,
		movementRepo: movementRepo,
	}
}

// CalculateForecast выполняет сбор складской аналитики и детерминированный расчет прогноза
func (s *forecastService) CalculateForecast(ctx context.Context, req ForecastRequest) (*domain.ForecastResult, error) {
	if err := s.validateRequest(ctx, req); err != nil {
		return nil, err
	}

	product, err := s.productRepo.GetProductBySKU(ctx, req.SKU)
	if err != nil {
		return nil, err
	}

	asOf := s.resolveAsOfDate(req)
	movements, currentStock, incomingQty, err := s.fetchForecastData(ctx, req, asOf)
	if err != nil {
		return nil, err
	}

	safetyDays := req.SafetyStockDays
	if safetyDays <= 0 {
		safetyDays = 14
	}

	return calc.CalculateForecast(calc.ForecastParams{
		Product:         *product,
		LocationID:      req.LocationID,
		AsOfDate:        asOf,
		PeriodFrom:      req.PeriodFrom,
		PeriodTo:        req.PeriodTo,
		SafetyStockDays: safetyDays,
		Movements:       movements,
		CurrentStock:    currentStock,
		IncomingQty:     incomingQty,
		UnitPrice:       req.UnitPrice,
	}), nil
}

func (s *forecastService) fetchForecastData(
	ctx context.Context,
	req ForecastRequest,
	asOf time.Time,
) ([]domain.Movement, decimal.Decimal, decimal.Decimal, error) {
	windowStart := asOf.AddDate(0, 0, -90)
	movements, err := s.movementRepo.GetMovementsInWindow(ctx, req.SKU, req.LocationID, windowStart, asOf)
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, fmt.Errorf("failed to fetch historical movements: %w", err)
	}

	currentStock, err := s.stockRepo.GetCurrentStock(ctx, req.SKU, req.LocationID)
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, fmt.Errorf("failed to get current stock: %w", err)
	}

	incomingQty, err := s.orderRepo.GetIncomingQuantity(ctx, req.SKU, req.LocationID)
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, fmt.Errorf("failed to get incoming quantity: %w", err)
	}

	return movements, currentStock, incomingQty, nil
}

func (s *forecastService) validateRequest(ctx context.Context, req ForecastRequest) error {
	if req.SKU == "" {
		return fmt.Errorf("sku is required: %w", domain.ErrNotFound)
	}
	if req.LocationID == "" {
		return fmt.Errorf("location is required: %w", domain.ErrNotFound)
	}
	if req.PeriodFrom.IsZero() || req.PeriodTo.IsZero() {
		return fmt.Errorf("period_from and period_to are required: %w", domain.ErrInvalidOperation)
	}
	if req.PeriodTo.Before(req.PeriodFrom) {
		return fmt.Errorf("period_to cannot be earlier than period_from: %w", domain.ErrInvalidOperation)
	}

	exists, err := s.productRepo.LocationExists(ctx, req.LocationID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("location %s not found: %w", req.LocationID, domain.ErrNotFound)
	}

	return nil
}

func (s *forecastService) resolveAsOfDate(req ForecastRequest) time.Time {
	if req.AsOfDate != nil && !req.AsOfDate.IsZero() {
		return *req.AsOfDate
	}
	return time.Now()
}
