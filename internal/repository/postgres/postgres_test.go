package postgres

import (
	"testing"
)

func TestRepositoryInterfaces(t *testing.T) {
	var _ MovementRepository = (*movementRepository)(nil)
	var _ StockRepository = (*stockRepository)(nil)
	var _ ProductRepository = (*productRepository)(nil)
	var _ OrderRepository = (*orderRepository)(nil)
	var _ TxManager = (*pgxTxManager)(nil)
}
