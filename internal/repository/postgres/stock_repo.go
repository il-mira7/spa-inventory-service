package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

const (
	movementBalanceCase = `
		CASE 
			WHEN operation_type IN ('receipt', 'return') THEN quantity
			WHEN operation_type IN ('consume', 'writeoff') THEN -quantity
			WHEN operation_type = 'correction' THEN quantity
			ELSE 0
		END
	`

	movementTableBalanceCase = `
		CASE 
			WHEN m.operation_type IN ('receipt', 'return') THEN m.quantity
			WHEN m.operation_type IN ('consume', 'writeoff') THEN -m.quantity
			WHEN m.operation_type = 'correction' THEN m.quantity
			ELSE 0
		END
	`
)

const stockSummaryQuery = `
	WITH current_stocks AS (
		SELECT 
			sku, 
			location_id,
			SUM(` + movementBalanceCase + `) AS stock
		FROM movements
		GROUP BY sku, location_id
	),
	consumption_90 AS (
		SELECT 
			sku,
			location_id,
			SUM(quantity) AS total_consumed
		FROM movements
		WHERE operation_type = 'consume'
		  AND operation_date >= (COALESCE((SELECT MAX(operation_date) FROM movements), CURRENT_TIMESTAMP) - INTERVAL '90 days')
		  AND operation_date <= COALESCE((SELECT MAX(operation_date) FROM movements), CURRENT_TIMESTAMP)
		GROUP BY sku, location_id
	),
	batch_balances AS (
		SELECT 
			m.sku,
			m.location_id,
			m.batch_id,
			b.expiry_date,
			SUM(` + movementTableBalanceCase + `) AS balance
		FROM movements m
		JOIN batches b ON b.id = m.batch_id AND b.sku = m.sku
		GROUP BY m.sku, m.location_id, m.batch_id, b.expiry_date
	),
	nearest_exp AS (
		SELECT 
			sku,
			location_id,
			MIN(expiry_date) AS nearest_expiry_date
		FROM batch_balances
		WHERE balance > 0 AND expiry_date >= CURRENT_DATE
		GROUP BY sku, location_id
	)
	SELECT 
		p.sku,
		p.name,
		l.id AS location_id,
		p.unit,
		COALESCE(cs.stock, 0.0000) AS current_stock,
		ROUND(COALESCE(c90.total_consumed, 0) / 90.0, 4) AS avg_daily_consumption,
		ROUND(
			COALESCE(cs.stock, 0.0000) / NULLIF(COALESCE(c90.total_consumed, 0) / 90.0, 0),
			1
		) AS stock_days,
		ne.nearest_expiry_date
	FROM products p
	CROSS JOIN locations l
	LEFT JOIN current_stocks cs ON cs.sku = p.sku AND cs.location_id = l.id
	LEFT JOIN consumption_90 c90 ON c90.sku = p.sku AND c90.location_id = l.id
	LEFT JOIN nearest_exp ne ON ne.sku = p.sku AND ne.location_id = l.id
	WHERE p.is_active = true
	  AND ($1::text IS NULL OR l.id = $1)
	ORDER BY p.sku ASC, l.id ASC;
`

const batchDetailQuery = `
	SELECT 
		m.location_id,
		l.name AS location_name,
		m.batch_id,
		b.expiry_date,
		COALESCE(b.purchase_price, 0.00),
		COALESCE(b.invoice_no, ''),
		COALESCE(b.received_date, '1970-01-01'::date),
		SUM(` + movementTableBalanceCase + `) AS balance
	FROM movements m
	JOIN locations l ON l.id = m.location_id
	LEFT JOIN batches b ON b.id = m.batch_id AND b.sku = m.sku
	WHERE m.sku = $1
	GROUP BY m.location_id, l.name, m.batch_id, b.expiry_date, b.purchase_price, b.invoice_no, b.received_date
	HAVING SUM(` + movementTableBalanceCase + `) > 0
	ORDER BY m.location_id ASC, b.expiry_date ASC NULLS LAST, m.batch_id ASC;
`

const activeBatchesQuery = `
	SELECT 
		m.batch_id,
		m.sku,
		m.location_id,
		SUM(` + movementTableBalanceCase + `) AS balance,
		b.expiry_date,
		COALESCE(b.purchase_price, 0.00),
		COALESCE(b.invoice_no, ''),
		COALESCE(b.received_date, '1970-01-01'::date)
	FROM movements m
	LEFT JOIN batches b ON b.id = m.batch_id AND b.sku = m.sku
	WHERE m.sku = $1 AND m.location_id = $2
	GROUP BY m.batch_id, m.sku, m.location_id, b.expiry_date, b.purchase_price, b.invoice_no, b.received_date
	HAVING SUM(` + movementTableBalanceCase + `) > 0
	ORDER BY b.expiry_date ASC NULLS LAST, m.batch_id ASC;
`

// StockRepository предоставляет методы для чтения текущих остатков, сроков годности и партий
type StockRepository interface {
	GetStockSummary(ctx context.Context, locationID *string) ([]domain.StockItem, error)
	GetStockDetailBySKU(ctx context.Context, sku string) (*domain.StockDetail, error)
	GetActiveBatchesBySKUAndLocation(ctx context.Context, sku, locationID string) ([]domain.BatchStock, error)
	GetCurrentStock(ctx context.Context, sku, locationID string) (decimal.Decimal, error)
}

type stockRepository struct {
	pool *pgxpool.Pool
}

// NewStockRepository создает новый экземпляр репозитория остатков
func NewStockRepository(pool *pgxpool.Pool) StockRepository {
	return &stockRepository{pool: pool}
}

// GetStockSummary формирует сводный отчет по остаткам (GET /api/stock).
func (r *stockRepository) GetStockSummary(ctx context.Context, locationID *string) ([]domain.StockItem, error) {
	engine := getQueryEngine(ctx, r.pool)
	locParam := normalizeLocationParam(locationID)

	rows, err := engine.Query(ctx, stockSummaryQuery, locParam)
	if err != nil {
		return nil, fmt.Errorf("failed to query stock summary: %w", err)
	}
	defer rows.Close()

	return scanStockSummaryRows(rows)
}

// GetStockDetailBySKU возвращает подробную детализацию по позициям с разбивкой по складам и партиям (GET /api/stock/{sku}).
func (r *stockRepository) GetStockDetailBySKU(ctx context.Context, sku string) (*domain.StockDetail, error) {
	engine := getQueryEngine(ctx, r.pool)

	header, err := fetchProductHeader(ctx, engine, sku)
	if err != nil {
		return nil, err
	}

	batchRows, err := fetchBatchDetailRows(ctx, engine, sku)
	if err != nil {
		return nil, err
	}

	header.Locations = groupBatchesByLocation(sku, batchRows)
	return header, nil
}

// GetActiveBatchesBySKUAndLocation возвращает партии с положительным остатком, отсортированные по сроку годности (FEFO)
func (r *stockRepository) GetActiveBatchesBySKUAndLocation(ctx context.Context, sku, locationID string) ([]domain.BatchStock, error) {
	engine := getQueryEngine(ctx, r.pool)

	rows, err := engine.Query(ctx, activeBatchesQuery, sku, locationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active batches for FEFO: %w", err)
	}
	defer rows.Close()

	return scanActiveBatches(rows)
}

// GetCurrentStock возвращает суммарный физический остаток позиции на объекте
func (r *stockRepository) GetCurrentStock(ctx context.Context, sku, locationID string) (decimal.Decimal, error) {
	engine := getQueryEngine(ctx, r.pool)
	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(%s), 0)
		FROM movements
		WHERE sku = $1 AND location_id = $2;
	`, movementBalanceCase)

	var stock decimal.Decimal
	if err := engine.QueryRow(ctx, query, sku, locationID).Scan(&stock); err != nil {
		return decimal.Zero, fmt.Errorf("failed to calculate current stock: %w", err)
	}

	return stock, nil
}

func normalizeLocationParam(loc *string) *string {
	if loc != nil && *loc != "" {
		return loc
	}
	return nil
}

func scanStockSummaryRows(rows pgx.Rows) ([]domain.StockItem, error) {
	items := make([]domain.StockItem, 0)
	for rows.Next() {
		var item domain.StockItem
		var stockDays *decimal.Decimal
		var nearestExp *time.Time

		if err := rows.Scan(
			&item.SKU,
			&item.Name,
			&item.LocationID,
			&item.Unit,
			&item.CurrentStock,
			&item.AvgDailyConsumption,
			&stockDays,
			&nearestExp,
		); err != nil {
			return nil, fmt.Errorf("failed to scan stock summary row: %w", err)
		}

		item.StockDays = stockDays
		item.NearestExpiryDate = nearestExp
		items = append(items, item)
	}

	return items, rows.Err()
}

func fetchProductHeader(ctx context.Context, engine QueryEngine, sku string) (*domain.StockDetail, error) {
	var product domain.StockDetail
	err := engine.QueryRow(ctx, `SELECT sku, name, unit FROM products WHERE sku = $1;`, sku).Scan(
		&product.SKU,
		&product.Name,
		&product.Unit,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find product by sku %s: %w", sku, err)
	}
	return &product, nil
}

type batchDetailRow struct {
	LocationID    string
	LocationName  string
	BatchID       string
	ExpiryDate    *time.Time
	PurchasePrice decimal.Decimal
	InvoiceNo     string
	ReceivedDate  time.Time
	Balance       decimal.Decimal
}

func fetchBatchDetailRows(ctx context.Context, engine QueryEngine, sku string) ([]batchDetailRow, error) {
	rows, err := engine.Query(ctx, batchDetailQuery, sku)
	if err != nil {
		return nil, fmt.Errorf("failed to query batch details: %w", err)
	}
	defer rows.Close()

	result := make([]batchDetailRow, 0)
	for rows.Next() {
		var row batchDetailRow
		if err := rows.Scan(
			&row.LocationID,
			&row.LocationName,
			&row.BatchID,
			&row.ExpiryDate,
			&row.PurchasePrice,
			&row.InvoiceNo,
			&row.ReceivedDate,
			&row.Balance,
		); err != nil {
			return nil, fmt.Errorf("failed to scan batch detail row: %w", err)
		}
		result = append(result, row)
	}

	return result, rows.Err()
}

func groupBatchesByLocation(sku string, batchRows []batchDetailRow) []domain.LocationStockBreakdown {
	locMap := make(map[string]*domain.LocationStockBreakdown)
	locOrder := make([]string, 0)

	for _, b := range batchRows {
		appendBatchToLocation(locMap, &locOrder, sku, b)
	}

	locations := make([]domain.LocationStockBreakdown, 0, len(locOrder))
	for _, locID := range locOrder {
		locations = append(locations, *locMap[locID])
	}
	return locations
}

func appendBatchToLocation(locMap map[string]*domain.LocationStockBreakdown, locOrder *[]string, sku string, b batchDetailRow) {
	loc, exists := locMap[b.LocationID]
	if !exists {
		loc = &domain.LocationStockBreakdown{
			LocationID:   b.LocationID,
			LocationName: b.LocationName,
			TotalStock:   decimal.Zero,
			Batches:      make([]domain.BatchStock, 0),
		}
		locMap[b.LocationID] = loc
		*locOrder = append(*locOrder, b.LocationID)
	}

	loc.TotalStock = loc.TotalStock.Add(b.Balance)
	loc.Batches = append(loc.Batches, domain.BatchStock{
		BatchID:       b.BatchID,
		SKU:           sku,
		LocationID:    b.LocationID,
		Quantity:      b.Balance,
		ExpiryDate:    b.ExpiryDate,
		PurchasePrice: b.PurchasePrice,
		InvoiceNo:     b.InvoiceNo,
		ReceivedDate:  b.ReceivedDate,
	})
}

func scanActiveBatches(rows pgx.Rows) ([]domain.BatchStock, error) {
	batches := make([]domain.BatchStock, 0)
	for rows.Next() {
		var bs domain.BatchStock
		if err := rows.Scan(
			&bs.BatchID,
			&bs.SKU,
			&bs.LocationID,
			&bs.Quantity,
			&bs.ExpiryDate,
			&bs.PurchasePrice,
			&bs.InvoiceNo,
			&bs.ReceivedDate,
		); err != nil {
			return nil, fmt.Errorf("failed to scan active batch: %w", err)
		}
		batches = append(batches, bs)
	}
	return batches, rows.Err()
}
