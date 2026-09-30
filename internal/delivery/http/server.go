package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/il-mira7/spa-inventory-service/internal/config"
	"go.uber.org/fx"
)

// Server инкапсулирует запуск и graceful shutdown веб-сервера
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// NewServer создает экземпляр HTTP сервера и регистрирует хуки жизненного цикла Fx
func NewServer(lc fx.Lifecycle, cfg *config.Config, logger *slog.Logger, router *chi.Mux) *Server {
	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.HTTPTimeout,
		WriteTimeout: cfg.HTTPTimeout + 5*time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s := &Server{
		httpServer: srv,
		logger:     logger,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			s.logger.Info("starting HTTP server", slog.String("addr", addr))
			go func() {
				if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					s.logger.Error("HTTP server stopped unexpectedly", slog.String("error", err.Error()))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			s.logger.Info("shutting down HTTP server gracefully")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return s.httpServer.Shutdown(shutdownCtx)
		},
	})

	return s
}
