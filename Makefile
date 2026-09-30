# ==============================================================================
# Makefile: Mountain & Sea Spa - Inventory & Procurement Service (ABS)
# ==============================================================================

APP_NAME     ?= spa-inventory-service
BIN_DIR      ?= bin
BINARY       ?= $(BIN_DIR)/api
MAIN_SRC     ?= cmd/api/main.go
DOCKER_IMAGE ?= $(APP_NAME):latest
GO           ?= go

.PHONY: help build run test test-coverage lint fmt tidy clean docker-build docker-up docker-down docker-logs

# Цель по умолчанию: справка
help: ## Отобразить список доступных команд
	@echo "Использование: make [цель]"
	@echo ""
	@echo "Доступные команды:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ------------------------------------------------------------------------------
# Сборка и запуск
# ------------------------------------------------------------------------------

build: ## Скомпилировать бинарный файл сервиса (bin/api)
	@echo "==> Сборка бинарного файла $(BINARY)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o $(BINARY) $(MAIN_SRC)
	@echo "==> Бинарный файл успешно собран: $(BINARY)"

run: ## Запустить сервис локально
	@echo "==> Запуск приложения через go run..."
	$(GO) run $(MAIN_SRC)

# ------------------------------------------------------------------------------
# Тестирование и качество кода
# ------------------------------------------------------------------------------

test: ## Запустить все тесты проекта
	@echo "==> Запуск тестов..."
	$(GO) test -v -count=1 ./...

test-coverage: ## Запустить тесты с генерацией отчета о покрытии
	@echo "==> Запуск тестов с расчетом покрытия..."
	@mkdir -p $(BIN_DIR)
	$(GO) test -count=1 -coverprofile=$(BIN_DIR)/coverage.out ./...
	$(GO) tool cover -func=$(BIN_DIR)/coverage.out
	$(GO) tool cover -html=$(BIN_DIR)/coverage.out -o $(BIN_DIR)/coverage.html
	@echo "==> HTML отчет успешно сгенерирован: $(BIN_DIR)/coverage.html"

test-e2e: ## Запустить сквозные E2E тесты против запущенного окружения
	@echo "==> Запуск сквозных E2E тестов..."
	docker run --rm --network spa-inventory-service_default -v "$$(pwd)":/app -w /app -e TEST_BASE_URL=http://api:8080 golang:alpine go test -v -count=1 ./test/e2e/...

lint: ## Проверить код линтером (go vet)
	@echo "==> Проверка кода через go vet..."
	$(GO) vet ./...

fmt: ## Отформатировать исходный код (gofmt)
	@echo "==> Форматирование исходного кода..."
	gofmt -s -w .

tidy: ## Обновить и очистить зависимости go.mod
	@echo "==> Очистка и синхронизация go.mod..."
	$(GO) mod tidy

# ------------------------------------------------------------------------------
# Docker и Docker Compose
# ------------------------------------------------------------------------------

docker-build: ## Собрать легковесный Docker-образ сервиса
	@echo "==> Сборка Docker-образа $(DOCKER_IMAGE)..."
	DOCKER_BUILDKIT=0 docker build -t $(DOCKER_IMAGE) .

docker-up: ## Запустить сервисы в Docker Compose (БД + API)
	@echo "==> Запуск окружения в Docker Compose..."
	docker compose up -d

up: docker-up ## Псевдоним для docker-up

docker-down: ## Остановить окружение Docker Compose и удалить тома
	@echo "==> Остановка Docker Compose..."
	docker compose down -v

down: docker-down ## Псевдоним для docker-down

docker-logs: ## Просмотр логов контейнеров
	@echo "==> Просмотр логов Docker Compose..."
	docker compose logs -f

logs: docker-logs ## Псевдоним для docker-logs

# ------------------------------------------------------------------------------
# Очистка
# ------------------------------------------------------------------------------

clean: ## Удалить скомпилированные бинарники и артефакты тестов
	@echo "==> Очистка артефактов сборки..."
	rm -rf $(BIN_DIR)
