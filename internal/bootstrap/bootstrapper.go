// internal/bootstrap/bootstrapper.go
package bootstrap

import (
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/mrPTqp/gofermart/internal/config"
	"github.com/mrPTqp/gofermart/internal/handler"
	"github.com/mrPTqp/gofermart/internal/service"
	"github.com/mrPTqp/gofermart/internal/storage/migrations"
	"github.com/mrPTqp/gofermart/internal/storage/postgres"
	"go.uber.org/zap"
)

type Bootstrapper struct {
	cfg    *config.Config
	logger *zap.Logger
	db     *sql.DB
}

func NewBootstrapper(cfg *config.Config, logger *zap.Logger) *Bootstrapper {
	return &Bootstrapper{cfg: cfg, logger: logger}
}

func (b *Bootstrapper) MustRun() *AppComponents {
	b.logger.Info("Starting application bootstrap...")

	b.logger.Info("Loading configuration")
	if b.cfg.DatabaseDsn == nil || *b.cfg.DatabaseDsn == "" {
		b.logger.Fatal("Database DSN is required")
	}

	b.logger.Info("Connecting to database")
	var err error
	b.db, err = sql.Open("pgx", *b.cfg.DatabaseDsn)
	if err != nil {
		b.logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	b.logger.Info("Applying migrations")
	if err := migrations.RunMigrations(*b.cfg.DatabaseDsn, b.logger); err != nil {
		b.logger.Panic("Failed to run migrations", zap.Error(err))
	}

	userStorage, err := postgres.NewUserStorage(b.db, b.logger)
	if err != nil {
		b.logger.Panic("Failed to init user storage", zap.Error(err))
	}
	orderStorage, err := postgres.NewOrderStorage(b.db, b.logger)
	if err != nil {
		b.logger.Panic("Failed to init order storage", zap.Error(err))
	}
	accountStorage, err := postgres.NewAccountStorage(b.db, b.logger)
	if err != nil {
		b.logger.Panic("Failed to init account storage", zap.Error(err))
	}

	userService := service.NewUserService(userStorage, b.cfg.JWTSecret, b.cfg.JWTTTL)
	orderService := service.NewOrderService(orderStorage)
	accountService := service.NewAccountService(accountStorage)

	accrualService, err := service.NewAccrualService("http://"+b.cfg.AccrualAddress.String(), orderStorage, accountService, b.logger)
	if err != nil {
		b.logger.Panic("Failed to create accrual service", zap.Error(err))
	}

	h := handler.NewGofermartHandler(userService, orderService, accountService, b.cfg, b.logger)

	return &AppComponents{
		Config:         b.cfg,
		Logger:         b.logger,
		DB:             b.db,
		Handler:        h,
		AccrualService: accrualService,
	}
}

type AppComponents struct {
	Config         *config.Config
	Logger         *zap.Logger
	DB             *sql.DB
	Handler        *handler.GofermartHandler
	AccrualService service.AccrualService
}
