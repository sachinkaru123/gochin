package commands

import (
	"fmt"
	"strconv"
	"github.com/spf13/cobra"
	"github.com/gochin/framework/pkg/config"
	"github.com/gochin/framework/internal/server"
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

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the development server",
		Long:  "Start the Gochin development server with specified host and port",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStartServer(host, port)
		},
	}

	// Add flags
	startCmd.Flags().StringVarP(&host, "host", "H", "", "Host to bind the server to")
	startCmd.Flags().StringVarP(&port, "port", "p", "", "Port to bind the server to")

	return startCmd
}

// runStartServer handles the logic for starting the server
func runStartServer(host, port string) error {
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