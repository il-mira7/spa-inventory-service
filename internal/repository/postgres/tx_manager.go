package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QueryEngine предоставляет унифицированный интерфейс для выполнения SQL-запросов
// как напрямую через пул соединений (*pgxpool.Pool), так и в рамках транзакции (pgx.Tx).
type QueryEngine interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type rowScanner interface {
	Scan(dest ...any) error
}

type txKey struct{}

// TxManager определяет контракт управления транзакциями (Unit of Work).
type TxManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type pgxTxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager создает новый экземпляр менеджера транзакций.
func NewTxManager(pool *pgxpool.Pool) TxManager {
	return &pgxTxManager{pool: pool}
}

// RunInTx выполняет переданную функцию внутри атомарной транзакции PostgreSQL.
// Если в процессе выполнения функции возвращается ошибка или происходит panic,
// транзакция автоматически откатывается (Rollback). При успешном завершении - фиксируется (Commit).
func (tm *pgxTxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	// Если мы уже находимся внутри активной транзакции, просто выполняем функцию
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := tm.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	txCtx := context.WithValue(ctx, txKey{}, tx)

	if err := fn(txCtx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// getQueryEngine извлекает активную транзакцию из контекста или возвращает пул соединений.
func getQueryEngine(ctx context.Context, pool *pgxpool.Pool) QueryEngine {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
