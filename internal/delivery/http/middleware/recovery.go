package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/il-mira7/spa-inventory-service/internal/delivery/http/dto"
)

// Recoverer перехватывает panic и возвращает структурированный JSON 500
func Recoverer(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					stack := string(debug.Stack())
					logger.Error("panic recovered in HTTP handler",
						slog.Any("panic", rvr),
						slog.String("stack", stack),
						slog.String("path", r.URL.Path),
					)

					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(dto.ErrorResponse{
						Error: fmt.Sprintf("internal server error: %v", rvr),
						Code:  http.StatusInternalServerError,
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
