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
)

// inspect logging logic
// add debug logs
// add readme (how are you processing balance? document it)
// tests
// remove comments
func main() {
	logger := logger.NewSugarLogger()

	logger.Info("try to create config")
	cfg := config.LoadConfig()
	logger.Infow("configuration created", "config", cfg)

	ctx := context.Background()

	var db *sql.DB
	if cfg.DatabaseDsn != nil && *cfg.DatabaseDsn != "" {
		var err error

		db, err = sql.Open("pgx", *cfg.DatabaseDsn)
		if err != nil {
			logger.Fatal("Failed to connect to DB: ", err)
		}
		defer db.Close()
	} else {
		log.Fatal("wrong DB DSN")
		os.Exit(0)
	}

	logger.Info("Applying database migrations...")
	if err := migrations.RunMigrations(*cfg.DatabaseDsn, logger); err != nil {
		logger.Panicf("Migration failed: %v", err)
	}
	logger.Info("Migrations applied successfully or no changes")

	ur, err := postgres.NewUserStorage(db, logger)
	if err != nil {
		logger.Panic("init postgres user repository error", err)
	}

	or, err := postgres.NewOrderStorage(db, logger)
	if err != nil {
		logger.Panic("init postgres order repository error", err)
	}

	ar, err := postgres.NewAccountStorage(db, logger)
	if err != nil {
		logger.Panic("init postgres account repository error", err)
	}

	us := service.NewUserService(ur, cfg.JWTSecret, cfg.JWTTTL)
	ors := service.NewOrderService(or)
	as := service.NewAccountService(ar)

	h := handler.NewGofermartHandler(us, ors, as, cfg, logger)

	accrualService, err := service.NewAccrualService("http://"+cfg.AccrualAddress.String(), or, as, logger)
	if err != nil {
		logger.Panicf("failed to create accrual service: %v", err)
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
		logger.Info("Closing storage connection...")
		if err := db.Close(); err != nil {
			logger.Errorf("Error closing storage connection: %v", err)
		} else {
			logger.Info("storage connection closed")
		}
	}
	logger.Info("Server gracefully shut down...")
}
