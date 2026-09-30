package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/calc"
	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/il-mira7/spa-inventory-service/internal/repository/postgres"
)

// AlertService определяет контракт формирования предупреждений и оценки рисков запасов
type AlertService interface {
	GetAlerts(ctx context.Context, locationID *string) ([]domain.Alert, error)
}

type alertService struct {
	productRepo postgres.ProductRepository
	stockRepo   postgres.StockRepository
}

// NewAlertService создает новый экземпляр сервиса складских предупреждений
func NewAlertService(
	productRepo postgres.ProductRepository,
	stockRepo postgres.StockRepository,
) AlertService {
	return &alertService{
		productRepo: productRepo,
		stockRepo:   stockRepo,
	}
}

// GetAlerts формирует реестр рисков по дефициту, истекающим срокам годности и неликвидам
func (s *alertService) GetAlerts(ctx context.Context, locationID *string) ([]domain.Alert, error) {
	if locationID != nil && *locationID != "" {
		exists, err := s.productRepo.LocationExists(ctx, *locationID)
		if err != nil {
			return nil, fmt.Errorf("failed to check location: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("location %s not found: %w", *locationID, domain.ErrNotFound)
		}
	}

	products, err := s.productRepo.GetAllActiveProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active products: %w", err)
	}

	stockItems, err := s.stockRepo.GetStockSummary(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stock summary: %w", err)
	}

	inputs, err := s.buildAlertInputs(ctx, products, stockItems)
	if err != nil {
		return nil, err
	}

	alerts := calc.EvaluateAlerts(inputs, 30)
	sortAlerts(alerts)

	return alerts, nil
}

func (s *alertService) buildAlertInputs(
	ctx context.Context,
	products []domain.Product,
	stockItems []domain.StockItem,
) ([]calc.StockAlertInput, error) {
	productMap := make(map[string]domain.Product, len(products))
	for _, p := range products {
		productMap[p.SKU] = p
	}

	asOf := time.Now()
	inputs := make([]calc.StockAlertInput, 0, len(stockItems))

	for _, item := range stockItems {
		prod, exists := productMap[item.SKU]
		if !exists {
			continue
		}

		batches, err := s.stockRepo.GetActiveBatchesBySKUAndLocation(ctx, item.SKU, item.LocationID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch active batches for alert: %w", err)
		}

		daysWithoutMovement := 0
		if item.AvgDailyConsumption.IsZero() && item.CurrentStock.GreaterThan(item.AvgDailyConsumption) {
			daysWithoutMovement = 90
		}

		inputs = append(inputs, calc.StockAlertInput{
			Product:             prod,
			LocationID:          item.LocationID,
			CurrentStock:        item.CurrentStock,
			AvgDailyConsumption: item.AvgDailyConsumption,
			StockDays:           item.StockDays,
			Batches:             batches,
			DaysWithoutMovement: daysWithoutMovement,
			AsOfDate:            asOf,
		})
	}

	return inputs, nil
}

func sortAlerts(alerts []domain.Alert) {
	priority := map[domain.AlertLevel]int{
		domain.AlertLevelCritical: 1,
		domain.AlertLevelWarning:  2,
		domain.AlertLevelInfo:     3,
	}

	sort.Slice(alerts, func(i, j int) bool {
		pI := priority[alerts[i].Level]
		pJ := priority[alerts[j].Level]
		if pI != pJ {
			return pI < pJ
		}
		if alerts[i].SKU != alerts[j].SKU {
			return alerts[i].SKU < alerts[j].SKU
		}
		return alerts[i].LocationID < alerts[j].LocationID
	})
}
