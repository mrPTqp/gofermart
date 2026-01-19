package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/mrPTqp/gofermart/internal/config"
	"github.com/mrPTqp/gofermart/internal/handler"
	mw "github.com/mrPTqp/gofermart/internal/middleware"
)

func StartGofermartServer(
	gh *handler.GofermartHandler,
	cfg *config.Config,
	logger *zap.Logger,
) *http.Server {
	r := chi.NewRouter()

	r.Use(mw.LoggingMiddleware(logger))
	r.Use(mw.GzipMiddleware())

	r.Post("/api/user/register", gh.Register)
	r.Post("/api/user/login", gh.Login)

	r.Group(func(r chi.Router) {
		r.Use(mw.AuthMiddleware(cfg.JWTSecret))

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
		logger.Info("Starting HTTP server", zap.String("address", cfg.Address.String()))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed to start", zap.Error(err))
		}
	}()

	return srv
}

func ShutdownGracefully(srv *http.Server, logger *zap.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown",
			zap.Error(err))
	} else {
		logger.Info("Server stopped gracefully")
	}
}
