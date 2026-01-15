// internal/app/app.go
package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/mrPTqp/gofermart/internal/config"
	"github.com/mrPTqp/gofermart/internal/handler"
	mw "github.com/mrPTqp/gofermart/internal/middleware"
)

// StartGofermartServer инициализирует маршруты, применяет middleware и запускает сервер
func StartGofermartServer(
	gh *handler.GofermartHandler,
	cfg *config.Config,
	logger *zap.SugaredLogger,
) *http.Server {
	r := chi.NewRouter()

	// Общие middleware
	r.Use(mw.LoggingMiddleware(logger))
	r.Use(mw.GzipMiddleware(logger))
	r.Use(middleware.Recoverer) // chi built-in panic recovery

	// Публичные маршруты (без авторизации)
	r.Post("/api/user/register", gh.Register)
	r.Post("/api/user/login", gh.Login)

	// Защищённые маршруты (требуют авторизации)
	r.Group(func(r chi.Router) {
		r.Use(mw.AuthMiddleware(logger))

		r.Post("/api/user/orders", gh.UploadOrder)
		r.Get("/api/user/orders", gh.GetOrders)
		r.Get("/api/user/balance", gh.GetBalance)
		r.Post("/api/user/balance/withdraw", gh.WithdrawBalance)
		r.Get("/api/user/withdrawals", gh.GetWithdrawals)
	})

	srv := &http.Server{
		Addr:         cfg.Address.String(),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		logger.Infof("Server is running on %s", cfg.Address.String())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed to start: %v", err)
		}
	}()

	return srv
}

// ShutdownGracefully корректно останавливает сервер
func ShutdownGracefully(srv *http.Server, logger *zap.SugaredLogger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	} else {
		logger.Info("Server stopped gracefully")
	}
}
