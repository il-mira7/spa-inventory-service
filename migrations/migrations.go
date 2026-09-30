package migrations

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var FS embed.FS

// Up запускает все применимые миграции схемы и начальных данных.
func Up(db *sql.DB) error {
	goose.SetBaseFS(FS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// Down откатывает последнюю миграцию.
func Down(db *sql.DB) error {
	goose.SetBaseFS(FS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Down(db, "."); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	return nil
}
