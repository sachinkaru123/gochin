package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	// Blank import: each file in app/Seeders registers itself from init().
	_ "github.com/gochin/framework/app/Seeders"
	"github.com/gochin/framework/pkg/database"
	"github.com/gochin/framework/pkg/orm"
)

func newDbSeedCommand() *cobra.Command {
	var only []string
	var list bool

	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Populate the database with seed data",
		Long:  "Run the seeders registered in app/Seeders. Seeders are safe to run repeatedly.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeeders(only, list)
		},
	}

	cmd.Flags().StringSliceVarP(&only, "only", "o", nil, "Run only the named seeders")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "List the registered seeders without running them")

	return cmd
}

func runSeeders(only []string, list bool) error {
	registered := orm.RegisteredSeeders()

	if list {
		if len(registered) == 0 {
			fmt.Println("No seeders registered. Create one with: gochin make seeder <Name>")
			return nil
		}
		fmt.Println("Registered seeders:")
		for _, s := range registered {
			fmt.Printf("  %-20s priority %d\n", s.Name, s.Priority)
		}
		return nil
	}

	if len(registered) == 0 {
		fmt.Println("No seeders registered. Create one with: gochin make seeder <Name>")
		return nil
	}

	if _, err := database.GetConnection(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	fmt.Println("Seeding database...")

	ran, err := orm.RunSeeders(context.Background(), only...)
	for _, name := range ran {
		fmt.Printf("Seeded: %s\n", name)
	}
	if err != nil {
		return err
	}

	if len(ran) == 0 {
		fmt.Println("Nothing to seed.")
		return nil
	}

	fmt.Printf("✅ %d seeder(s) completed\n", len(ran))
	return nil
}
