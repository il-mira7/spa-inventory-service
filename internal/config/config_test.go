package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig_Defaults(t *testing.T) {
	cfg, err := config.NewConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 8080, cfg.HTTPPort)
	assert.Equal(t, 15*time.Second, cfg.HTTPTimeout)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, int32(25), cfg.DBPoolMaxConns)
}

func TestNewConfig_EnvOverride(t *testing.T) {
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("HTTP_TIMEOUT", "30s")

	cfg, err := config.NewConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 9090, cfg.HTTPPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 30*time.Second, cfg.HTTPTimeout)
}

func TestMain(m *testing.M) {
	// Очищаем окружение от случайных переменных перед тестами
	_ = os.Unsetenv("HTTP_PORT")
	_ = os.Unsetenv("LOG_LEVEL")
	_ = os.Unsetenv("HTTP_TIMEOUT")
	os.Exit(m.Run())
}
