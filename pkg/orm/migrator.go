package orm

import (
	"database/sql"
	"fmt"
	"sort"
)

// Migrator applies and rolls back registered migrations against db, tracking
// applied versions in a schema_migrations table.
type Migrator struct {
	db *sql.DB
}

// NewMigrator creates a Migrator bound to db.
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// EnsureSchemaTable creates the schema_migrations tracking table if it
// doesn't already exist.
func (m *Migrator) EnsureSchemaTable() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    VARCHAR(14) PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

// appliedVersions returns the set of migration versions already applied.
func (m *Migrator) appliedVersions() (map[string]bool, error) {
	rows, err := m.db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

// sorted returns every registered migration sorted ascending by version,
// erroring if two migrations share a version.
func sortedMigrations() ([]Migration, error) {
	migrations := registered()
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	seen := map[string]bool{}
	for _, mig := range migrations {
		if seen[mig.Version] {
			return nil, fmt.Errorf("orm: duplicate migration version %q", mig.Version)
		}
		seen[mig.Version] = true
	}

	return migrations, nil
}

// Pending returns every registered migration not yet applied, sorted
// ascending by version.
func (m *Migrator) Pending() ([]Migration, error) {
	migrations, err := sortedMigrations()
	if err != nil {
		return nil, err
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, mig := range migrations {
		if !applied[mig.Version] {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}

// Up applies every pending migration, in version order, each in its own
// transaction. It stops and returns an error on the first failure, leaving
// already-applied migrations in place.
func (m *Migrator) Up() ([]Migration, error) {
	pending, err := m.Pending()
	if err != nil {
		return nil, err
	}

	var applied []Migration
	for _, mig := range pending {
		if err := m.applyUp(mig); err != nil {
			return applied, fmt.Errorf("orm: migration %s_%s failed: %w", mig.Version, mig.Name, err)
		}
		applied = append(applied, mig)
	}

	return applied, nil
}

func (m *Migrator) applyUp(mig Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}

	if err := mig.Up(tx); err != nil {
		tx.Rollback()
		return err
	}

	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		mig.Version, mig.Name,
	); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Down rolls back the single most-recently-applied migration. It returns
// (nil, nil) if no migrations have been applied.
func (m *Migrator) Down() (*Migration, error) {
	migrations, err := sortedMigrations()
	if err != nil {
		return nil, err
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return nil, err
	}

	var target *Migration
	for i := len(migrations) - 1; i >= 0; i-- {
		if applied[migrations[i].Version] {
			target = &migrations[i]
			break
		}
	}
	if target == nil {
		return nil, nil
	}

	tx, err := m.db.Begin()
	if err != nil {
		return nil, err
	}

	if err := target.Down(tx); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("orm: rollback of migration %s_%s failed: %w", target.Version, target.Name, err)
	}

	if _, err := tx.Exec(`DELETE FROM schema_migrations WHERE version = $1`, target.Version); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return target, nil
}
