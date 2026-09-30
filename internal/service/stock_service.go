package service

import (
	"context"
	"fmt"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/il-mira7/spa-inventory-service/internal/repository/postgres"
)

// StockService предоставляет методы чтения складских остатков и деталей партий
type StockService interface {
	GetStockSummary(ctx context.Context, locationID *string) ([]domain.StockItem, error)
	GetStockDetail(ctx context.Context, sku string) (*domain.StockDetail, error)
}

type stockService struct {
	stockRepo   postgres.StockRepository
	productRepo postgres.ProductRepository
}

// NewStockService создает экземпляр сервиса остатков
func NewStockService(
	stockRepo postgres.StockRepository,
	productRepo postgres.ProductRepository,
) StockService {
	return &stockService{
		stockRepo:   stockRepo,
		productRepo: productRepo,
	}
}

// GetStockSummary возвращает сводный реестр остатков с опциональной фильтрацией по складу
func (s *stockService) GetStockSummary(ctx context.Context, locationID *string) ([]domain.StockItem, error) {
	if locationID != nil && *locationID != "" {
		exists, err := s.productRepo.LocationExists(ctx, *locationID)
		if err != nil {
			return nil, fmt.Errorf("failed to check location existence: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("location %s not found: %w", *locationID, domain.ErrNotFound)
		}
	}

	return s.stockRepo.GetStockSummary(ctx, locationID)
}

// GetStockDetail возвращает детализацию по позициям с разбивкой по объектам и активным партиям
func (s *stockService) GetStockDetail(ctx context.Context, sku string) (*domain.StockDetail, error) {
	if sku == "" {
		return nil, fmt.Errorf("sku cannot be empty: %w", domain.ErrNotFound)
	}

	return s.stockRepo.GetStockDetailBySKU(ctx, sku)
}
