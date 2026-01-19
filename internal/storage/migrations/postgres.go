// internal/storage/migrations/postgres.go
package migrations

import (
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

func RunMigrations(dsn string, sugar *zap.Logger) error {
	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		sugar.Error("Failed to initialize migration tool", zap.Error(err))
		return err
	}
	defer m.Close()

	current, _, _ := m.Version()
	sugar.Info("Current database schema version", zap.Uint("version", current))

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			sugar.Info("No migrations to apply")
			return nil
		}

		sugar.Error("Migration failed, attempting rollback",
			zap.Error(err))
		if rollbackErr := m.Down(); rollbackErr != nil {
			sugar.Error("Rollback after migration failure failed",
				zap.Error(rollbackErr))
		} else {
			sugar.Info("Rollback after migration failure succeeded")
		}
		return err
	}

	newVersion, _, _ := m.Version()
	if newVersion > current {
		sugar.Info("Database schema migrated successfully",
			zap.Uint("from", current),
			zap.Uint("to", newVersion))
	} else {
		sugar.Info("Database schema is up to date")
	}

	return nil
}
