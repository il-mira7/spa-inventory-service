package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MovementFilter задает критерии фильтрации журнала движений
type MovementFilter struct {
	SKU           *string
	LocationID    *string
	OperationType *domain.OperationType
	FromDate      *time.Time
	ToDate        *time.Time
}

// MovementRepository интерфейс доступа к журналу движений
type MovementRepository interface {
	InsertProcessedDocument(ctx context.Context, m domain.Movement) error
	InsertMovements(ctx context.Context, movements []domain.Movement) ([]int64, error)
	GetMovements(ctx context.Context, filter MovementFilter, limit, offset int) ([]domain.Movement, int64, error)
	GetMovementsBySKUAndLocation(ctx context.Context, sku, locationID string) ([]domain.Movement, error)
	GetMovementsInWindow(ctx context.Context, sku, locationID string, from, to time.Time) ([]domain.Movement, error)
}

type movementRepository struct {
	pool *pgxpool.Pool
}

// NewMovementRepository создает новый экземпляр репозитория движений
func NewMovementRepository(pool *pgxpool.Pool) MovementRepository {
	return &movementRepository{pool: pool}
}

// InsertProcessedDocument фиксирует номер документа в processed_documents.
// Если документ с таким номером уже существует в БД, возвращает domain.ErrDuplicateDocument (409 Conflict).
func (r *movementRepository) InsertProcessedDocument(ctx context.Context, m domain.Movement) error {
	engine := getQueryEngine(ctx, r.pool)

	query := `
		INSERT INTO processed_documents (document_no, operation_type, operation_date, sku, location_id)
		VALUES ($1, $2, $3, $4, $5);
	`

	_, err := engine.Exec(ctx, query,
		m.DocumentNo,
		string(m.OperationType),
		m.OperationDate,
		m.SKU,
		m.LocationID,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return domain.ErrDuplicateDocument
		}
		return fmt.Errorf("failed to insert processed document: %w", err)
	}

	return nil
}

// InsertMovements пакетно сохраняет строки движений в журнал movements
func (r *movementRepository) InsertMovements(ctx context.Context, movements []domain.Movement) ([]int64, error) {
	if len(movements) == 0 {
		return nil, nil
	}

	engine := getQueryEngine(ctx, r.pool)
	ids := make([]int64, 0, len(movements))

	query := `
		INSERT INTO movements (document_no, operation_date, sku, location_id, operation_type, quantity, batch_id, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id;
	`

	for _, m := range movements {
		var id int64
		err := engine.QueryRow(ctx, query,
			m.DocumentNo,
			m.OperationDate,
			m.SKU,
			m.LocationID,
			string(m.OperationType),
			m.Quantity,
			m.BatchID,
			m.Note,
		).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to insert movement record: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// GetMovements выполняет выборку списка движений с динамической фильтрацией и пагинацией.
func (r *movementRepository) GetMovements(ctx context.Context, filter MovementFilter, limit, offset int) ([]domain.Movement, int64, error) {
	engine := getQueryEngine(ctx, r.pool)
	whereSQL, args := buildMovementFilterSQL(filter)
	limit, offset = normalizePagination(limit, offset)

	query := buildPaginatedMovementsQuery(whereSQL, len(args)+1, len(args)+2)
	queryArgs := append(args, limit, offset)

	rows, err := engine.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query movements: %w", err)
	}
	defer rows.Close()

	items, total, err := scanMovementsWithTotal(rows)
	if err != nil {
		return nil, 0, err
	}

	if len(items) == 0 && offset > 0 {
		total, err = countTotalMovements(ctx, engine, whereSQL, args)
		if err != nil {
			return nil, 0, err
		}
	}

	return items, total, nil
}

// GetMovementsBySKUAndLocation возвращает все движения по позиции на объекте для динамического расчета остатка
func (r *movementRepository) GetMovementsBySKUAndLocation(ctx context.Context, sku, locationID string) ([]domain.Movement, error) {
	engine := getQueryEngine(ctx, r.pool)

	query := `
		SELECT id, document_no, operation_date, sku, location_id, operation_type, quantity, batch_id, COALESCE(note, ''), created_at
		FROM movements
		WHERE sku = $1 AND location_id = $2
		ORDER BY operation_date ASC, id ASC;
	`

	rows, err := engine.Query(ctx, query, sku, locationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get movements by sku and location: %w", err)
	}
	defer rows.Close()

	return scanMovements(rows)
}

// GetMovementsInWindow возвращает движения за заданный временной интервал
func (r *movementRepository) GetMovementsInWindow(ctx context.Context, sku, locationID string, from, to time.Time) ([]domain.Movement, error) {
	engine := getQueryEngine(ctx, r.pool)

	query := `
		SELECT id, document_no, operation_date, sku, location_id, operation_type, quantity, batch_id, COALESCE(note, ''), created_at
		FROM movements
		WHERE sku = $1 AND location_id = $2 AND operation_date >= $3 AND operation_date <= $4
		ORDER BY operation_date ASC, id ASC;
	`

	rows, err := engine.Query(ctx, query, sku, locationID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get movements in window: %w", err)
	}
	defer rows.Close()

	return scanMovements(rows)
}

func buildMovementFilterSQL(filter MovementFilter) (string, []any) {
	clauses := make([]string, 0)
	args := make([]any, 0)
	argIdx := 1

	addClause := func(clause string, val any) {
		clauses = append(clauses, fmt.Sprintf(clause, argIdx))
		args = append(args, val)
		argIdx++
	}

	if filter.SKU != nil && *filter.SKU != "" {
		addClause("m.sku = $%d", *filter.SKU)
	}
	if filter.LocationID != nil && *filter.LocationID != "" {
		addClause("m.location_id = $%d", *filter.LocationID)
	}
	if filter.OperationType != nil && filter.OperationType.IsValid() {
		addClause("m.operation_type = $%d", string(*filter.OperationType))
	}
	if filter.FromDate != nil {
		addClause("m.operation_date >= $%d", *filter.FromDate)
	}
	if filter.ToDate != nil {
		addClause("m.operation_date <= $%d", *filter.ToDate)
	}

	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func normalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func buildPaginatedMovementsQuery(whereSQL string, limitArgIdx, offsetArgIdx int) string {
	return fmt.Sprintf(`
		SELECT 
			m.id, 
			m.document_no, 
			m.operation_date, 
			m.sku, 
			m.location_id, 
			m.operation_type, 
			m.quantity, 
			m.batch_id, 
			COALESCE(m.note, ''), 
			m.created_at,
			COUNT(*) OVER() AS total_count
		FROM movements m
		%s
		ORDER BY m.operation_date DESC, m.id DESC
		LIMIT $%d OFFSET $%d;
	`, whereSQL, limitArgIdx, offsetArgIdx)
}

func scanMovementsWithTotal(rows pgx.Rows) ([]domain.Movement, int64, error) {
	items := make([]domain.Movement, 0)
	var total int64 = 0

	for rows.Next() {
		var m domain.Movement
		var opType string
		var count int64

		if err := rows.Scan(
			&m.ID,
			&m.DocumentNo,
			&m.OperationDate,
			&m.SKU,
			&m.LocationID,
			&opType,
			&m.Quantity,
			&m.BatchID,
			&m.Note,
			&m.CreatedAt,
			&count,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan movement row: %w", err)
		}

		m.OperationType = domain.OperationType(opType)
		items = append(items, m)
		total = count
	}

	return items, total, rows.Err()
}

func countTotalMovements(ctx context.Context, engine QueryEngine, whereSQL string, args []any) (int64, error) {
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM movements m %s;", whereSQL)
	var exactCount int64
	if err := engine.QueryRow(ctx, countQuery, args...).Scan(&exactCount); err != nil {
		return 0, fmt.Errorf("failed to count total movements: %w", err)
	}
	return exactCount, nil
}

func scanMovement(scanner rowScanner, m *domain.Movement) error {
	var opType string
	if err := scanner.Scan(
		&m.ID,
		&m.DocumentNo,
		&m.OperationDate,
		&m.SKU,
		&m.LocationID,
		&opType,
		&m.Quantity,
		&m.BatchID,
		&m.Note,
		&m.CreatedAt,
	); err != nil {
		return err
	}
	m.OperationType = domain.OperationType(opType)
	return nil
}

func scanMovements(rows pgx.Rows) ([]domain.Movement, error) {
	items := make([]domain.Movement, 0)
	for rows.Next() {
		var m domain.Movement
		if err := scanMovement(rows, &m); err != nil {
			return nil, fmt.Errorf("failed to scan movement: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}
