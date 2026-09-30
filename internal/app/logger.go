package app

import (
	"log/slog"
	"os"
	"strings"

	"github.com/il-mira7/spa-inventory-service/internal/config"
	"go.uber.org/fx/fxevent"
)

// NewLogger создает структурированный JSON-логгер slog на основе конфигурации.
func NewLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	// Устанавливаем в качестве глобального логгера по умолчанию
	slog.SetDefault(logger)

	return logger
}

// FxSlogLogger реализует интерфейс fxevent.Logger для интеграции Uber FX со slog.
type FxSlogLogger struct {
	logger *slog.Logger
}

// NewFxLogger создает адаптер fxevent.Logger.
func NewFxLogger(logger *slog.Logger) fxevent.Logger {
	return &FxSlogLogger{logger: logger}
}

// LogEvent перенаправляет внутренние события Uber FX в slog.
func (l *FxSlogLogger) LogEvent(event fxevent.Event) {
	switch e := event.(type) {
	case *fxevent.OnStartExecuting:
		l.logger.Debug("fx: executing OnStart hook",
			slog.String("caller", e.CallerName),
			slog.String("function", e.FunctionName),
		)
	case *fxevent.OnStartExecuted:
		if e.Err != nil {
			l.logger.Error("fx: OnStart hook failed",
				slog.String("caller", e.CallerName),
				slog.String("function", e.FunctionName),
				slog.String("error", e.Err.Error()),
			)
		} else {
			l.logger.Debug("fx: OnStart hook executed",
				slog.String("caller", e.CallerName),
				slog.String("function", e.FunctionName),
				slog.Duration("runtime", e.Runtime),
			)
		}
	case *fxevent.OnStopExecuting:
		l.logger.Debug("fx: executing OnStop hook",
			slog.String("caller", e.CallerName),
			slog.String("function", e.FunctionName),
		)
	case *fxevent.OnStopExecuted:
		if e.Err != nil {
			l.logger.Error("fx: OnStop hook failed",
				slog.String("caller", e.CallerName),
				slog.String("function", e.FunctionName),
				slog.String("error", e.Err.Error()),
			)
		} else {
			l.logger.Debug("fx: OnStop hook executed",
				slog.String("caller", e.CallerName),
				slog.String("function", e.FunctionName),
				slog.Duration("runtime", e.Runtime),
			)
		}
	case *fxevent.Supplied:
		if e.Err != nil {
			l.logger.Error("fx: error supplying type",
				slog.String("type", e.TypeName),
				slog.String("error", e.Err.Error()),
			)
		}
	case *fxevent.Provided:
		if e.Err != nil {
			l.logger.Error("fx: error providing type",
				slog.String("error", e.Err.Error()),
			)
		}
	case *fxevent.Invoked:
		if e.Err != nil {
			l.logger.Error("fx: invoke failed",
				slog.String("function", e.FunctionName),
				slog.String("error", e.Err.Error()),
			)
		}
	case *fxevent.Started:
		if e.Err != nil {
			l.logger.Error("fx: application failed to start",
				slog.String("error", e.Err.Error()),
			)
		} else {
			l.logger.Info("fx: application started successfully")
		}
	case *fxevent.Stopped:
		if e.Err != nil {
			l.logger.Error("fx: application failed to stop gracefully",
				slog.String("error", e.Err.Error()),
			)
		} else {
			l.logger.Info("fx: application stopped gracefully")
		}
	case *fxevent.RollingBack:
		l.logger.Warn("fx: rolling back startup",
			slog.String("error", e.StartErr.Error()),
		)
	case *fxevent.RolledBack:
		if e.Err != nil {
			l.logger.Error("fx: rollback failed",
				slog.String("error", e.Err.Error()),
			)
		}
	}
}
