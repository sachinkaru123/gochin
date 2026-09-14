package cli

import (
	"fmt"
	"os"

	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"

	"github.com/gochin/framework/bootstrap"
	"github.com/gochin/framework/internal/commands"
	"github.com/gochin/framework/pkg/config"
	"github.com/gochin/framework/pkg/logs"
)

// Build-time variables (can be set with -ldflags)
var (
	Version   = "1.0.0"   // Default version
	BuildDate = "unknown" // Set at build time
	GitCommit = "unknown" // Set at build time
)

var rootCmd = &cobra.Command{
	Use:   "gochin",
	Short: "Gochin is a modern Go API framework",
	Long: `Gochin is a fast and flexible Go-based API framework that provides
	a powerful CLI for rapid application development, including scaffolding,
	server management, and database operations.`,
	Version: Version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	myFigure := figure.NewFigure("GOCHIN", "", true)
	myFigure.Print()
	fmt.Println("------------------------------------------------------")
	fmt.Println("------------------------------------------------------")
	fmt.Println("")
	fmt.Println("🚀 Welcome to the GoChin Framework")

	// CLI commands log to the same files as the server, so a seeder or
	// migration run leaves a record. A logging failure must not stop the
	// command itself.
	if cfg, err := config.Load(); err == nil {
		if err := bootstrap.ConfigureLogging(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		defer logs.Close()
	}

	return rootCmd.Execute()
}

func init() {
	// Initialize and register all commands
	registerCommands()
}

// registerCommands registers all available commands with the CLI
func registerCommands() {
	// Register run command and its subcommands
	rootCmd.AddCommand(commands.NewRunCommand())

	// Register make command and its subcommands
	rootCmd.AddCommand(commands.NewMakeCommand())

	// Register db command and its subcommands
	rootCmd.AddCommand(commands.NewDbCommand())
	rootCmd.AddCommand(commands.NewRouteCommand())
}
