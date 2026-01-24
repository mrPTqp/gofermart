package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrPTqp/gofermart/internal/app"
	"github.com/mrPTqp/gofermart/internal/bootstrap"
	"github.com/mrPTqp/gofermart/internal/config"
	"github.com/mrPTqp/gofermart/internal/logger"
)

func main() {
	logger := logger.NewLogger()
	defer func() {
		_ = logger.Sync()
	}()

	cfg := config.LoadConfig()

	bootstrapper := bootstrap.NewBootstrapper(cfg, logger)
	components := bootstrapper.MustRun()

	application := app.NewApp(components)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go application.Run()

	<-ctx.Done()
	log.Println("Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	application.Shutdown(shutdownCtx)
	log.Println("Application stopped")
}
