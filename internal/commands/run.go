package commands

import (
	"fmt"
	"github.com/gochin/framework/internal/server"
	"github.com/gochin/framework/pkg/config"
	"github.com/spf13/cobra"
	"strconv"
)

// NewRunCommand creates the run command with its subcommands
func NewRunCommand() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run server operations",
		Long:  "Run various server operations like starting the development server",
	}

	// Add subcommands
	runCmd.AddCommand(newRunStartCommand())

	return runCmd
}

// newRunStartCommand creates the "run start" subcommand
func newRunStartCommand() *cobra.Command {
	var port string
	var host string
	var skipDbTest bool

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the development server",
		Long:  "Start the Gochin development server with specified host and port",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStartServer(host, port, skipDbTest)
		},
	}

	// Add flags
	startCmd.Flags().StringVarP(&host, "host", "H", "", "Host to bind the server to")
	startCmd.Flags().StringVarP(&port, "port", "p", "", "Port to bind the server to")
	startCmd.Flags().BoolVar(&skipDbTest, "skip-db-test", false, "Skip database connection test on startup")

	return startCmd
}

// runStartServer handles the logic for starting the server
func runStartServer(host, port string, skipDbTest bool) error {
	fmt.Printf("Starting Gochin server...\n")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override config with command line flags if provided
	if host != "" {
		config.SetHost(host)
		cfg.Server.Host = host
	}

	if port != "" {
		if portInt, err := strconv.Atoi(port); err == nil {
			config.SetPort(portInt)
			cfg.Server.Port = portInt
		} else {
			return fmt.Errorf("invalid port number: %s", port)
		}
	}

	fmt.Printf("App: %s (%s)\n", cfg.App.Name, cfg.App.Environment)
	fmt.Printf("Server starting on %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Debug mode: %t\n", cfg.App.Debug)
	fmt.Println()

	// Test database connection during startup (unless skipped)
	if !skipDbTest {
		fmt.Println("🔌 Testing database connection...")
		if err := cfg.Database.TestConnection(); err != nil {
			fmt.Printf("⚠️  Database connection failed: %v\n", err)
			fmt.Println("   Server will start without database connectivity")
			fmt.Println("   Check your database settings in .env file")
			fmt.Println("   Use --skip-db-test flag to skip this check")
		} else {
			fmt.Println("✅ Database connection successful!")
		}
		fmt.Println()
	}

	fmt.Println("🚀 Gochin server is ready!")
	fmt.Printf("   ➜ Local:   http://%s\n", config.GetServerAddr())

	if cfg.Server.Host != "localhost" && cfg.Server.Host != "127.0.0.1" {
		fmt.Printf("   ➜ Network: http://%s\n", config.GetServerAddr())
	}

	fmt.Println()

	// Create and start the HTTP server
	srv := server.New(cfg.Server.Host, cfg.Server.Port)
	return srv.Start()
}
