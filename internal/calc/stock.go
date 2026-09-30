package calc

import (
	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/shopspring/decimal"
)

// AggregateStock вычисляет текущий остаток из последовательности движений товара (Ledger Pattern).
// receipt (+), return (+), consume (-), writeoff (-), correction (+/-)
func AggregateStock(movements []domain.Movement) decimal.Decimal {
	total := decimal.Zero
	for _, m := range movements {
		total = total.Add(m.SignedQuantity())
	}
	return total
}

type batchKey struct {
	batchID    string
	sku        string
	locationID string
}

// AggregateBatchStocks группирует движения по партиям и вычисляет текущий остаток каждой партии.
func AggregateBatchStocks(movements []domain.Movement, batchMap map[string]domain.Batch) []domain.BatchStock {
	balanceMap := calculateBatchBalances(movements)
	result := buildBatchStockList(balanceMap, batchMap)
	SortBatchStocksFEFO(result)
	return result
}

func calculateBatchBalances(movements []domain.Movement) map[batchKey]decimal.Decimal {
	balances := make(map[batchKey]decimal.Decimal)
	for _, m := range movements {
		k := batchKey{batchID: m.BatchID, sku: m.SKU, locationID: m.LocationID}
		balances[k] = balances[k].Add(m.SignedQuantity())
	}
	return balances
}

func buildBatchStockList(balances map[batchKey]decimal.Decimal, batchMap map[string]domain.Batch) []domain.BatchStock {
	result := make([]domain.BatchStock, 0, len(balances))
	for k, balance := range balances {
		bs := domain.BatchStock{
			BatchID:    k.batchID,
			SKU:        k.sku,
			LocationID: k.locationID,
			Quantity:   balance,
		}
		if b, exists := batchMap[k.batchID]; exists {
			bs.ExpiryDate = b.ExpiryDate
			bs.PurchasePrice = b.PurchasePrice
			bs.InvoiceNo = b.InvoiceNo
			bs.ReceivedDate = b.ReceivedDate
		}
		result = append(result, bs)
	}
	return result
}
