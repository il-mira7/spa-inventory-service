package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/app"
)

func main() {
	application := app.NewApp()
	if err := application.Err(); err != nil {
		renderFatalError(err)
		os.Exit(1)
	}

	startCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := application.Start(startCtx); err != nil {
		renderFatalError(err)
		os.Exit(1)
	}

	<-application.Done()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer stopCancel()

	if err := application.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка при остановке сервиса: %v\n", err)
	}
}

func renderFatalError(err error) {
	errStr := err.Error()

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "\033[1;31m================================================================================")
	fmt.Fprintln(os.Stderr, "❌ КРИТИЧЕСКАЯ ОШИБКА ЗАПУСКА СЕРВИСА (STARTUP FAILED)")
	fmt.Fprintln(os.Stderr, "================================================================================\033[0m")

	if strings.Contains(errStr, "database ping failed") || strings.Contains(errStr, "connection refused") {
		fmt.Fprintln(os.Stderr, "\033[1;33mПричина:\033[0m Не удалось установить соединение с базой данных PostgreSQL.")
		fmt.Fprintln(os.Stderr, "         Приложение не может стартовать без проверки соединения и применения миграций.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "\033[1;36mДетали ошибки:\033[0m")
		fmt.Fprintf(os.Stderr, "  %s\n", extractRootCause(errStr))
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "\033[1;32m💡 Решение:\033[0m")
		fmt.Fprintln(os.Stderr, "  1. Запустите базу данных в Docker Compose:")
		fmt.Fprintln(os.Stderr, "     \033[1m$ docker compose up -d db\033[0m")
		fmt.Fprintln(os.Stderr, "     После этого повторите запуск: \033[1m$ make run\033[0m")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  2. Либо запустите весь стек целиком (API + БД) через Docker:")
		fmt.Fprintln(os.Stderr, "     \033[1m$ make docker-up\033[0m")
	} else {
		fmt.Fprintln(os.Stderr, "\033[1;33mПричина:\033[0m Сбой инициализации компонентов приложения.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "\033[1;36mДетали ошибки:\033[0m")
		fmt.Fprintf(os.Stderr, "  %s\n", extractRootCause(errStr))
	}

	fmt.Fprintln(os.Stderr, "\033[1;31m================================================================================\033[0m")
	fmt.Fprintln(os.Stderr, "")
}

func extractRootCause(errStr string) string {
	// Извлекаем конкретное сообщение об ошибке, отсекая внутренние префиксы графа Uber FX
	if idx := strings.LastIndex(errStr, "received non-nil error from function"); idx != -1 {
		sub := errStr[idx:]
		if colonIdx := strings.Index(sub, "): "); colonIdx != -1 {
			return strings.TrimSpace(sub[colonIdx+3:])
		}
	}
	return errStr
}
