// File: services/chat/migrations/migrate.go
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
)

//go:embed *.sql
var migrationFiles embed.FS

// Migration represents a database migration
type Migration struct {
	Version   string
	Name      string
	Filename  string
	SQL       string
	Applied   bool
	AppliedAt *string
}

// MigrationManager handles database migrations
type MigrationManager struct {
	db  *database.DB
	log *logger.Logger
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *database.DB, log *logger.Logger) *MigrationManager {
	return &MigrationManager{
		db:  db,
		log: log,
	}
}

// RunMigrations executes all pending migrations
func (m *MigrationManager) RunMigrations() error {
	m.log.Info("Starting database migrations...")

	// Create migrations tracking table
	if err := m.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get all migration files
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Check which migrations have been applied
	if err := m.checkAppliedMigrations(migrations); err != nil {
		return fmt.Errorf("failed to check applied migrations: %w", err)
	}

	// Apply pending migrations
	pendingCount := 0
	for _, migration := range migrations {
		if !migration.Applied {
			m.log.Infof("Applying migration: %s", migration.Name)
			if err := m.applyMigration(migration); err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", migration.Name, err)
			}
			pendingCount++
		}
	}

	if pendingCount == 0 {
		m.log.Info("No pending migrations found")
	} else {
		m.log.Infof("Applied %d migrations successfully", pendingCount)
	}

	return nil
}

// createMigrationsTable creates the migrations tracking table
func (m *MigrationManager) createMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	return m.db.Exec(query).Error
}

// loadMigrations loads all migration files from the embedded filesystem
func (m *MigrationManager) loadMigrations() ([]*Migration, error) {
	var migrations []*Migration

	err := fs.WalkDir(migrationFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		content, err := migrationFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}

		filename := filepath.Base(path)
		parts := strings.SplitN(filename, "_", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid migration filename format: %s", filename)
		}

		version := parts[0]
		name := strings.TrimSuffix(parts[1], ".sql")

		migration := &Migration{
			Version:  version,
			Name:     name,
			Filename: filename,
			SQL:      string(content),
		}

		migrations = append(migrations, migration)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// checkAppliedMigrations checks which migrations have already been applied
func (m *MigrationManager) checkAppliedMigrations(migrations []*Migration) error {
	rows, err := m.db.Raw("SELECT version, applied_at FROM schema_migrations").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	appliedMigrations := make(map[string]string)
	for rows.Next() {
		var version, appliedAt string
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return err
		}
		appliedMigrations[version] = appliedAt
	}

	// Mark applied migrations
	for _, migration := range migrations {
		if appliedAt, exists := appliedMigrations[migration.Version]; exists {
			migration.Applied = true
			migration.AppliedAt = &appliedAt
		}
	}

	return nil
}

// applyMigration applies a single migration
func (m *MigrationManager) applyMigration(migration *Migration) error {
	// Start transaction
	tx := m.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Execute migration SQL
	if err := tx.Exec(migration.SQL).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record migration as applied
	if err := tx.Exec(
		"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
		migration.Version,
		migration.Name,
	).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit migration transaction: %w", err)
	}

	return nil
}

// GetAppliedMigrations returns a list of applied migrations
func (m *MigrationManager) GetAppliedMigrations() ([]*Migration, error) {
	var migrations []*Migration

	rows, err := m.db.Raw(`
		SELECT version, name, applied_at 
		FROM schema_migrations 
		ORDER BY version
	`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var migration Migration
		var appliedAt string
		if err := rows.Scan(&migration.Version, &migration.Name, &appliedAt); err != nil {
			return nil, err
		}
		migration.Applied = true
		migration.AppliedAt = &appliedAt
		migrations = append(migrations, &migration)
	}

	return migrations, nil
}
