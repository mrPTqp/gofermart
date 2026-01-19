package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/mrPTqp/gofermart/internal/app"
	"github.com/mrPTqp/gofermart/internal/config"
	"github.com/mrPTqp/gofermart/internal/handler"
	"github.com/mrPTqp/gofermart/internal/logger"
	"github.com/mrPTqp/gofermart/internal/service"
	"github.com/mrPTqp/gofermart/internal/storage/migrations"
	"github.com/mrPTqp/gofermart/internal/storage/postgres"
	"go.uber.org/zap"
)

func main() {
	logger := logger.NewLogger()
	defer logger.Sync()

	logger.Info("Initializing configuration")
	cfg := config.LoadConfig()
	logger.Info("Configuration loaded",
		zap.String("address", cfg.Address.String()),
		zap.String("database_dsn", *cfg.DatabaseDsn),
		zap.String("accrual_address", cfg.AccrualAddress.String()),
		zap.Duration("order_check_interval", cfg.OrderCheckInterval),
	)

	ctx := context.Background()

	var db *sql.DB
	if cfg.DatabaseDsn != nil && *cfg.DatabaseDsn != "" {
		var err error
		db, err = sql.Open("pgx", *cfg.DatabaseDsn)
		if err != nil {
			logger.Fatal("Failed to connect to database", zap.Error(err))
		}
		defer db.Close()
	} else {
		log.Fatal("Database DSN is required")
		os.Exit(1)
	}

	logger.Info("Applying database migrations...")
	if err := migrations.RunMigrations(*cfg.DatabaseDsn, logger); err != nil {
		logger.Panic("Database migrations failed", zap.Error(err))
	}
	logger.Info("Migrations applied successfully or already up to date")

	ur, err := postgres.NewUserStorage(db, logger)
	if err != nil {
		logger.Panic("Failed to initialize user storage", zap.Error(err))
	}

	or, err := postgres.NewOrderStorage(db, logger)
	if err != nil {
		logger.Panic("Failed to initialize order storage", zap.Error(err))
	}

	ar, err := postgres.NewAccountStorage(db, logger)
	if err != nil {
		logger.Panic("Failed to initialize account storage", zap.Error(err))
	}

	us := service.NewUserService(ur, cfg.JWTSecret, cfg.JWTTTL)
	ors := service.NewOrderService(or)
	as := service.NewAccountService(ar)

	h := handler.NewGofermartHandler(us, ors, as, cfg, logger)

	accrualService, err := service.NewAccrualService("http://"+cfg.AccrualAddress.String(), or, as, logger)
	if err != nil {
		logger.Panic("Failed to create accrual service", zap.Error(err))
	}

	go func() {
		ticker := time.NewTicker(cfg.OrderCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				accrualService.ProcessOrders(ctx)
			}
		}
	}()

	srv := app.StartGofermartServer(h, cfg, logger)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("Shutting down server gracefully...")
	app.ShutdownGracefully(srv, logger)

	if db != nil {
		logger.Info("Closing database connection...")
		if err := db.Close(); err != nil {
			logger.Error("Error closing database connection", zap.Error(err))
		} else {
			logger.Info("Database connection closed")
		}
	}
	logger.Info("Server shutdown complete")
}
