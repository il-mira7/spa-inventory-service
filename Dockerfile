# ==============================================================================
# Multi-stage Dockerfile for Mountain & Sea Spa Inventory Service
# Target image size: ~25 MB, non-root user, minimal Alpine 3.20 base
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Builder
# ------------------------------------------------------------------------------
FROM golang:alpine AS builder

WORKDIR /app

# Установка ca-certificates для загрузки модулей
RUN apk add --no-cache ca-certificates git

# Кеширование зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Компиляция статического бинарника без CGO с усечением отладочных символов (-s -w)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api cmd/api/main.go

# ------------------------------------------------------------------------------
# Stage 2: Minimal Runtime
# ------------------------------------------------------------------------------
FROM alpine:3.20

# Установка актуальных корневых сертификатов и данных таймзон
RUN apk --no-cache add ca-certificates tzdata

# Создание системного непривилегированного пользователя appuser (UID/GID 10001)
RUN addgroup -g 10001 -S appuser && \
    adduser -u 10001 -S appuser -G appuser

WORKDIR /app

# Копирование скомпилированного бинарника из builder
COPY --from=builder /bin/api /bin/api

# Переключение на непривилегированного пользователя
USER appuser:appuser

# Экспорт HTTP-порта
EXPOSE 8080

# Точка входа
ENTRYPOINT ["/bin/api"]
