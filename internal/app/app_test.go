package app_test

import (
	"testing"

	"github.com/il-mira7/spa-inventory-service/internal/app"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
)

func TestNewApp_GraphValidation(t *testing.T) {
	// ValidateApp проверяет корректность графа зависимостей DI без запуска приложения и без подключения к сети/БД
	err := fx.ValidateApp(app.Options()...)
	assert.NoError(t, err)
}
