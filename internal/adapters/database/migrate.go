package database

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/pkg/logger"
)

// RunMigrations executes all pending database migrations
func RunMigrations(db *sql.DB, migrationsPath string) error {
	logger.Info("running database migrations",
		zap.String("path", migrationsPath),
	)

	// Create postgres driver instance
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Get current version
	currentVersion, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	if dirty {
		logger.Warn("database is in dirty state, attempting to force version",
			zap.Uint("version", currentVersion),
		)
		if err := m.Force(int(currentVersion)); err != nil {
			return fmt.Errorf("failed to force version: %w", err)
		}
	}

	logger.Info("current migration version",
		zap.Uint("version", currentVersion),
		zap.Bool("dirty", dirty),
	)

	// Run migrations
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("no new migrations to apply")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Get new version
	newVersion, _, err := m.Version()
	if err != nil {
		return fmt.Errorf("failed to get new migration version: %w", err)
	}

	logger.Info("migrations completed successfully",
		zap.Uint("old_version", currentVersion),
		zap.Uint("new_version", newVersion),
	)

	return nil
}

// RollbackMigration rolls back the last applied migration
func RollbackMigration(db *sql.DB, migrationsPath string) error {
	logger.Info("rolling back last migration",
		zap.String("path", migrationsPath),
	)

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	currentVersion, _, err := m.Version()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if err := m.Steps(-1); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	logger.Info("migration rolled back successfully",
		zap.Uint("from_version", currentVersion),
		zap.Uint("to_version", currentVersion-1),
	)

	return nil
}

// GetMigrationVersion returns current migration version
func GetMigrationVersion(db *sql.DB, migrationsPath string) (uint, bool, error) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return 0, false, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return 0, false, fmt.Errorf("failed to get version: %w", err)
	}

	return version, dirty, nil
}
