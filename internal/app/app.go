// internal/app/app.go
package app

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/mrPTqp/gofermart/internal/bootstrap"
	mw "github.com/mrPTqp/gofermart/internal/middleware"
)

type App struct {
	cfg      *bootstrap.AppComponents
	server   *http.Server
	logger   *zap.Logger
	db       *sql.DB
	ticker   *time.Ticker
	cancel   context.CancelFunc // Для отмены фоновых задач
	shutdown sync.Once
}

func NewApp(components *bootstrap.AppComponents) *App {
	r := chi.NewRouter()
	r.Use(mw.LoggingMiddleware(components.Logger))
	r.Use(mw.GzipMiddleware())

	h := components.Handler
	cfg := components.Config

	r.Post("/api/user/register", h.Register)
	r.Post("/api/user/login", h.Login)

	r.Group(func(r chi.Router) {
		r.Use(mw.AuthMiddleware(cfg.JWTSecret))
		r.Post("/api/user/orders", h.UploadOrder)
		r.Get("/api/user/orders", h.GetOrders)
		r.Get("/api/user/balance", h.GetBalance)
		r.Post("/api/user/balance/withdraw", h.WithdrawBalance)
		r.Get("/api/user/withdrawals", h.GetWithdrawals)
	})

	server := &http.Server{
		Addr:         cfg.Address.String(),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	return &App{
		cfg:    components,
		server: server,
		logger: components.Logger,
		db:     components.DB,
		ticker: time.NewTicker(cfg.OrderCheckInterval),
	}
}

func (a *App) Run() {
	a.logger.Info("Starting HTTP server", zap.String("address", a.cfg.Config.Address.String()))

	// Запуск HTTP-сервера
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Fatal("HTTP server failed to start", zap.Error(err))
		}
	}()

	// Контекст для фоновых задач
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	// Запуск фонового процесса проверки заказов
	go a.runBackgroundJobs(ctx)
}

// runBackgroundJobs — фоновая проверка статусов заказов через accrual-сервис
func (a *App) runBackgroundJobs(ctx context.Context) {
	a.logger.Info("Background order processing ticker started", zap.Duration("interval", a.cfg.Config.OrderCheckInterval))

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Background job ticker stopped", zap.String("reason", ctx.Err().Error()))
			return
		case <-a.ticker.C:
			a.logger.Debug("Processing orders via accrual service")
			a.cfg.AccrualService.ProcessOrders(context.Background())
		}
	}
}

// Shutdown — останавливает сервер, тикер и закрывает соединение с БД
func (a *App) Shutdown(ctx context.Context) {
	a.shutdown.Do(func() {
		a.logger.Info("Shutting down application gracefully...")

		// Отменяем контекст фоновых задач
		if a.cancel != nil {
			a.cancel()
		}

		// Останавливаем тикер
		a.ticker.Stop()
		a.logger.Debug("Ticker stopped")

		// Останавливаем HTTP-сервер
		if err := a.server.Shutdown(ctx); err != nil {
			a.logger.Error("HTTP server shutdown error", zap.Error(err))
		} else {
			a.logger.Info("HTTP server stopped gracefully")
		}

		if err := a.db.Close(); err != nil {
			a.logger.Error("Database connection close error", zap.Error(err))
		} else {
			a.logger.Info("Database connection closed")
		}
	})
}
