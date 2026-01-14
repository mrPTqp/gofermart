package migrations

import (
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

func RunMigrations(dsn string, sugar *zap.SugaredLogger) error {
	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		return err
	}
	defer m.Close()

	current, _, _ := m.Version()
	sugar.Infof("Current migration version: %d", current)

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			sugar.Info("No migrations to apply")
			return nil
		}

		sugar.Errorf("Migration error: %v. Attempting rollback...", err)
		if rollbackErr := m.Down(); rollbackErr != nil {
			sugar.Errorf("Rollback after migration failure failed: %v", rollbackErr)
		} else {
			sugar.Info("Rollback after migration failure succeeded")
		}

		return err
	}

	newVersion, _, _ := m.Version()
	if newVersion > current {
		sugar.Infof("Successfully migrated to version %d", newVersion)
	} else {
		sugar.Info("No new migrations found")
	}

	return nil
}
