package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const productColumns = `
	sku, name, category, unit, package_size, min_order_qty, lead_time_days, 
	default_supplier_id, last_purchase_price, is_active, created_at
`

const batchColumns = `
	id, sku, expiry_date, purchase_price, invoice_no, received_date, created_at
`

// ProductRepository предоставляет доступ к справочникам товаров, складов, поставщиков и партий
type ProductRepository interface {
	GetProductBySKU(ctx context.Context, sku string) (*domain.Product, error)
	LockProductBySKU(ctx context.Context, sku string) error
	GetAllActiveProducts(ctx context.Context) ([]domain.Product, error)
	LocationExists(ctx context.Context, locationID string) (bool, error)
	BatchExists(ctx context.Context, batchID, sku string) (bool, error)
	GetBatch(ctx context.Context, batchID, sku string) (*domain.Batch, error)
	UpsertBatch(ctx context.Context, batch domain.Batch) error
	GetLocations(ctx context.Context) ([]domain.Location, error)
	GetSuppliers(ctx context.Context) ([]domain.Supplier, error)
}

type productRepository struct {
	pool *pgxpool.Pool
}

// NewProductRepository создает новый экземпляр репозитория товаров
func NewProductRepository(pool *pgxpool.Pool) ProductRepository {
	return &productRepository{pool: pool}
}

// GetProductBySKU возвращает товар по его коду (SKU).
func (r *productRepository) GetProductBySKU(ctx context.Context, sku string) (*domain.Product, error) {
	engine := getQueryEngine(ctx, r.pool)
	query := fmt.Sprintf("SELECT %s FROM products WHERE sku = $1;", productColumns)

	var p domain.Product
	if err := scanProduct(engine.QueryRow(ctx, query, sku), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get product by sku %s: %w", sku, err)
	}

	return &p, nil
}

// LockProductBySKU выполняет пессимистическую блокировку строки товара в рамках текущей транзакции.
func (r *productRepository) LockProductBySKU(ctx context.Context, sku string) error {
	engine := getQueryEngine(ctx, r.pool)
	query := `SELECT sku FROM products WHERE sku = $1 FOR UPDATE;`

	var lockedSKU string
	if err := engine.QueryRow(ctx, query, sku).Scan(&lockedSKU); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to lock product sku %s for update: %w", sku, err)
	}

	return nil
}

// GetAllActiveProducts возвращает список всех активных товаров номенклатуры
func (r *productRepository) GetAllActiveProducts(ctx context.Context) ([]domain.Product, error) {
	engine := getQueryEngine(ctx, r.pool)
	query := fmt.Sprintf("SELECT %s FROM products WHERE is_active = true ORDER BY sku ASC;", productColumns)

	rows, err := engine.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active products: %w", err)
	}
	defer rows.Close()

	return scanProducts(rows)
}

// LocationExists проверяет наличие указанного склада / локации
func (r *productRepository) LocationExists(ctx context.Context, locationID string) (bool, error) {
	engine := getQueryEngine(ctx, r.pool)

	var exists bool
	err := engine.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM locations WHERE id = $1);`, locationID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check location existence: %w", err)
	}

	return exists, nil
}

// BatchExists проверяет существование партии в таблице batches
func (r *productRepository) BatchExists(ctx context.Context, batchID, sku string) (bool, error) {
	engine := getQueryEngine(ctx, r.pool)

	var exists bool
	err := engine.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM batches WHERE id = $1 AND sku = $2);`, batchID, sku).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check batch existence: %w", err)
	}

	return exists, nil
}

// GetBatch возвращает данные партии
func (r *productRepository) GetBatch(ctx context.Context, batchID, sku string) (*domain.Batch, error) {
	engine := getQueryEngine(ctx, r.pool)
	query := fmt.Sprintf("SELECT %s FROM batches WHERE id = $1 AND sku = $2;", batchColumns)

	var b domain.Batch
	if err := scanBatch(engine.QueryRow(ctx, query, batchID, sku), &b); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get batch %s for sku %s: %w", batchID, sku, err)
	}

	return &b, nil
}

// UpsertBatch регистрирует новую партию или обновляет данные существующей (при поступлении товара)
func (r *productRepository) UpsertBatch(ctx context.Context, batch domain.Batch) error {
	engine := getQueryEngine(ctx, r.pool)

	query := `
		INSERT INTO batches (id, sku, expiry_date, purchase_price, invoice_no, received_date)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id, sku) DO UPDATE SET
			expiry_date = COALESCE(EXCLUDED.expiry_date, batches.expiry_date),
			purchase_price = CASE WHEN EXCLUDED.purchase_price > 0 THEN EXCLUDED.purchase_price ELSE batches.purchase_price END,
			invoice_no = CASE WHEN EXCLUDED.invoice_no <> '' THEN EXCLUDED.invoice_no ELSE batches.invoice_no END;
	`

	_, err := engine.Exec(ctx, query,
		batch.ID,
		batch.SKU,
		batch.ExpiryDate,
		batch.PurchasePrice,
		batch.InvoiceNo,
		batch.ReceivedDate,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert batch %s for sku %s: %w", batch.ID, batch.SKU, err)
	}

	return nil
}

// GetLocations возвращает перечень всех складов/объектов
func (r *productRepository) GetLocations(ctx context.Context) ([]domain.Location, error) {
	engine := getQueryEngine(ctx, r.pool)
	query := `SELECT id, name, COALESCE(address, ''), created_at FROM locations ORDER BY id ASC;`

	rows, err := engine.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query locations: %w", err)
	}
	defer rows.Close()

	locations := make([]domain.Location, 0)
	for rows.Next() {
		var l domain.Location
		if err := rows.Scan(&l.ID, &l.Name, &l.Address, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}
		locations = append(locations, l)
	}

	return locations, rows.Err()
}

// GetSuppliers возвращает список всех поставщиков
func (r *productRepository) GetSuppliers(ctx context.Context) ([]domain.Supplier, error) {
	engine := getQueryEngine(ctx, r.pool)
	query := `SELECT id, name, COALESCE(contact_person, ''), COALESCE(phone, ''), COALESCE(email, ''), created_at FROM suppliers ORDER BY id ASC;`

	rows, err := engine.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query suppliers: %w", err)
	}
	defer rows.Close()

	suppliers := make([]domain.Supplier, 0)
	for rows.Next() {
		var s domain.Supplier
		if err := rows.Scan(&s.ID, &s.Name, &s.ContactPerson, &s.Phone, &s.Email, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan supplier: %w", err)
		}
		suppliers = append(suppliers, s)
	}

	return suppliers, rows.Err()
}

func scanProduct(scanner rowScanner, p *domain.Product) error {
	return scanner.Scan(
		&p.SKU,
		&p.Name,
		&p.Category,
		&p.Unit,
		&p.PackageSize,
		&p.MinOrderQty,
		&p.LeadTimeDays,
		&p.DefaultSupplierID,
		&p.LastPurchasePrice,
		&p.IsActive,
		&p.CreatedAt,
	)
}

func scanProducts(rows pgx.Rows) ([]domain.Product, error) {
	products := make([]domain.Product, 0)
	for rows.Next() {
		var p domain.Product
		if err := scanProduct(rows, &p); err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func scanBatch(scanner rowScanner, b *domain.Batch) error {
	return scanner.Scan(
		&b.ID,
		&b.SKU,
		&b.ExpiryDate,
		&b.PurchasePrice,
		&b.InvoiceNo,
		&b.ReceivedDate,
		&b.CreatedAt,
	)
}
