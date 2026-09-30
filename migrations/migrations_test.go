package migrations_test

import (
	"io/fs"
	"testing"

	"github.com/il-mira7/spa-inventory-service/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedMigrations(t *testing.T) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	assert.Contains(t, names, "00001_init_schema.sql")
	assert.Contains(t, names, "00002_seed_data.sql")

	for _, name := range []string{"00001_init_schema.sql", "00002_seed_data.sql"} {
		content, err := fs.ReadFile(migrations.FS, name)
		require.NoError(t, err)
		assert.NotEmpty(t, content, "Migration file %s must not be empty", name)
		assert.Contains(t, string(content), "-- +goose Up", "Migration file %s must contain goose Up directive", name)
		assert.Contains(t, string(content), "-- +goose Down", "Migration file %s must contain goose Down directive", name)
	}
}
