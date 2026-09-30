package swagger

import (
	"embed"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed swagger.yaml index.html
var swaggerFS embed.FS

// RegisterRoutes регистрирует эндпоинты Swagger UI и спецификации OpenAPI через embed.FS
func RegisterRoutes(r chi.Router) {
	// Редирект /docs на /swagger/
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
	r.Get("/docs/*", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})

	// Редирект /swagger на /swagger/
	r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})

	// HTML интерфейс Swagger UI
	r.Get("/swagger/", func(w http.ResponseWriter, r *http.Request) {
		content, err := swaggerFS.ReadFile("index.html")
		if err != nil {
			http.Error(w, "Swagger UI not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	})

	// Спецификация OpenAPI в формате YAML
	r.Get("/swagger/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		content, err := swaggerFS.ReadFile("swagger.yaml")
		if err != nil {
			http.Error(w, "OpenAPI specification not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	})

	r.Get("/swagger/*", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "openapi.yaml") || strings.HasSuffix(r.URL.Path, "swagger.yaml") {
			content, err := swaggerFS.ReadFile("swagger.yaml")
			if err != nil {
				http.Error(w, "OpenAPI specification not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(content)
			return
		}
		http.Redirect(w, r, "/swagger/", http.StatusFound)
	})
}
