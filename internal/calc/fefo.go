package calc

import (
	"sort"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/shopspring/decimal"
)

// FEFOAllocation представляет объем списания из конкретной партии
type FEFOAllocation struct {
	BatchID       string          `json:"batch_id"`
	AllocatedQty  decimal.Decimal `json:"allocated_qty"`
	ExpiryDate    *time.Time      `json:"expiry_date,omitempty"`
	PurchasePrice decimal.Decimal `json:"purchase_price"`
}

// AllocateFEFO реализует алгоритм списания по принципу First Expired, First Out.
// Декомпозирован на чистые шаги: валидация -> фильтрация -> проверка остатка -> сортировка -> распределение.
func AllocateFEFO(
	batches []domain.BatchStock,
	requestedQty decimal.Decimal,
	asOf time.Time,
	sku string,
	locationID string,
) ([]FEFOAllocation, decimal.Decimal, error) {
	if requestedQty.LessThanOrEqual(decimal.Zero) {
		return nil, decimal.Zero, domain.ErrInvalidQuantity
	}

	eligibleBatches, totalAvailable := FilterEligibleBatches(batches, asOf)

	if requestedQty.GreaterThan(totalAvailable) {
		return nil, totalAvailable, domain.NewInsufficientStockError(sku, locationID, requestedQty, totalAvailable)
	}

	SortBatchStocksFEFO(eligibleBatches)
	allocations := DistributeConsumption(eligibleBatches, requestedQty)

	return allocations, totalAvailable, nil
}

// FilterEligibleBatches отбирает партии с положительным остатком, исключая просроченные (expiry_date <= asOf)
func FilterEligibleBatches(batches []domain.BatchStock, asOf time.Time) ([]domain.BatchStock, decimal.Decimal) {
	eligible := make([]domain.BatchStock, 0, len(batches))
	totalAvailable := decimal.Zero

	for _, b := range batches {
		if b.IsExpired(asOf) || b.Quantity.LessThanOrEqual(decimal.Zero) {
			continue
		}
		eligible = append(eligible, b)
		totalAvailable = totalAvailable.Add(b.Quantity)
	}

	return eligible, totalAvailable
}

// SortBatchStocksFEFO выполняет единую каноническую сортировку партий по правилу FEFO:
// 1. Партии с указанным сроком годности идут раньше партий без срока
// 2. Ближайший срок годности идет первым (expiry_date ASC)
// 3. При равных сроках годности - раньше поступившая партия (received_date ASC)
func SortBatchStocksFEFO(batches []domain.BatchStock) {
	sort.Slice(batches, func(i, j int) bool {
		if batches[i].ExpiryDate != nil && batches[j].ExpiryDate == nil {
			return true
		}
		if batches[i].ExpiryDate == nil && batches[j].ExpiryDate != nil {
			return false
		}
		if batches[i].ExpiryDate != nil && batches[j].ExpiryDate != nil {
			if !batches[i].ExpiryDate.Equal(*batches[j].ExpiryDate) {
				return batches[i].ExpiryDate.Before(*batches[j].ExpiryDate)
			}
		}
		return batches[i].ReceivedDate.Before(batches[j].ReceivedDate)
	})
}

// DistributeConsumption последовательно распределяет требуемый объем по отсортированным партиям
func DistributeConsumption(batches []domain.BatchStock, requestedQty decimal.Decimal) []FEFOAllocation {
	remaining := requestedQty
	allocations := make([]FEFOAllocation, 0, len(batches))

	for _, b := range batches {
		if remaining.LessThanOrEqual(decimal.Zero) {
			break
		}

		take := decimal.Min(b.Quantity, remaining)
		allocations = append(allocations, FEFOAllocation{
			BatchID:       b.BatchID,
			AllocatedQty:  take,
			ExpiryDate:    b.ExpiryDate,
			PurchasePrice: b.PurchasePrice,
		})

		remaining = remaining.Sub(take)
	}

	return allocations
}
