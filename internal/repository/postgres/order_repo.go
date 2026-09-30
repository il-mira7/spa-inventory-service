package postgres

import (
	"context"
	"fmt"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

const openOrderColumns = `id, sku, location_id, supplier_id, quantity, order_date, expected_delivery_date, status, created_at`

// OrderRepository предоставляет доступ к открытым заказам и поставкам в пути
type OrderRepository interface {
	GetIncomingQuantity(ctx context.Context, sku, locationID string) (decimal.Decimal, error)
	GetOpenOrdersBySKUAndLocation(ctx context.Context, sku, locationID string) ([]domain.OpenOrder, error)
	CreateOrder(ctx context.Context, order domain.OpenOrder) error
}

type orderRepository struct {
	pool *pgxpool.Pool
}

// NewOrderRepository создает экземпляр репозитория заказов
func NewOrderRepository(pool *pgxpool.Pool) OrderRepository {
	return &orderRepository{pool: pool}
}

// GetIncomingQuantity возвращает суммарный объем товара, находящегося в пути (статус in_transit).
func (r *orderRepository) GetIncomingQuantity(ctx context.Context, sku, locationID string) (decimal.Decimal, error) {
	engine := getQueryEngine(ctx, r.pool)

	query := `
		SELECT COALESCE(SUM(quantity), 0)
		FROM open_orders
		WHERE sku = $1 AND location_id = $2 AND status = 'in_transit';
	`

	var qty decimal.Decimal
	err := engine.QueryRow(ctx, query, sku, locationID).Scan(&qty)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get incoming quantity for sku %s at %s: %w", sku, locationID, err)
	}

	return qty, nil
}

// GetOpenOrdersBySKUAndLocation возвращает список активных заказов в пути
func (r *orderRepository) GetOpenOrdersBySKUAndLocation(ctx context.Context, sku, locationID string) ([]domain.OpenOrder, error) {
	engine := getQueryEngine(ctx, r.pool)
	query := fmt.Sprintf(`
		SELECT %s
		FROM open_orders
		WHERE sku = $1 AND location_id = $2 AND status = 'in_transit'
		ORDER BY expected_delivery_date ASC;
	`, openOrderColumns)

	rows, err := engine.Query(ctx, query, sku, locationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query open orders: %w", err)
	}
	defer rows.Close()

	return scanOpenOrders(rows)
}

// CreateOrder создает новый заказ поставщику
func (r *orderRepository) CreateOrder(ctx context.Context, order domain.OpenOrder) error {
	engine := getQueryEngine(ctx, r.pool)

	query := `
		INSERT INTO open_orders (id, sku, location_id, supplier_id, quantity, order_date, expected_delivery_date, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
	`

	_, err := engine.Exec(ctx, query,
		order.ID,
		order.SKU,
		order.LocationID,
		order.SupplierID,
		order.Quantity,
		order.OrderDate,
		order.ExpectedDeliveryDate,
		order.Status,
	)

	if err != nil {
		return fmt.Errorf("failed to create open order %s: %w", order.ID, err)
	}

	return nil
}

func scanOpenOrders(rows pgx.Rows) ([]domain.OpenOrder, error) {
	orders := make([]domain.OpenOrder, 0)
	for rows.Next() {
		var o domain.OpenOrder
		if err := rows.Scan(
			&o.ID,
			&o.SKU,
			&o.LocationID,
			&o.SupplierID,
			&o.Quantity,
			&o.OrderDate,
			&o.ExpectedDeliveryDate,
			&o.Status,
			&o.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan open order: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}
