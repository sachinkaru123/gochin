package commands

import (
	"fmt"

	_ "github.com/gochin/framework/app/Migrations"
	"github.com/gochin/framework/pkg/database"
	"github.com/gochin/framework/pkg/orm"
	"github.com/spf13/cobra"
)

// NewDbCommand creates the db command with its subcommands
func NewDbCommand() *cobra.Command {
	dbCmd := &cobra.Command{
		Use:   "db",
		Short: "Database operations",
		Long:  "Perform various database operations like migrations, seeding, etc.",
	}

	// Add subcommands
	dbCmd.AddCommand(newDbMigrateCommand())
	dbCmd.AddCommand(newDbSeedCommand())

	return dbCmd
}

// newDbMigrateCommand creates the "db:migrate" subcommand
func newDbMigrateCommand() *cobra.Command {
	var rollback bool

	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Long:  "Run pending database migrations or rollback to previous state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrations(rollback)
		},
	}

	// Add flags
	migrateCmd.Flags().BoolVarP(&rollback, "rollback", "r", false, "Rollback the last migration")

	return migrateCmd
}

// runMigrations handles the logic for running database migrations
func runMigrations(rollback bool) error {
	db, err := database.GetConnection()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	migrator := orm.NewMigrator(db)
	if err := migrator.EnsureSchemaTable(); err != nil {
		return fmt.Errorf("failed to prepare schema_migrations table: %w", err)
	}

	if rollback {
		fmt.Println("Rolling back last migration...")

		migration, err := migrator.Down()
		if err != nil {
			return fmt.Errorf("rollback failed: %w", err)
		}
		if migration == nil {
			fmt.Println("No migrations to roll back.")
			return nil
		}

		fmt.Printf("Rolled back: %s_%s\n", migration.Version, migration.Name)
		return nil
	}

	fmt.Println("Running database migrations...")

	applied, err := migrator.Up()
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	if len(applied) == 0 {
		fmt.Println("Nothing to migrate. Already up to date.")
		return nil
	}

	for _, migration := range applied {
		fmt.Printf("Migrated: %s_%s\n", migration.Version, migration.Name)
	}

	return nil
}
