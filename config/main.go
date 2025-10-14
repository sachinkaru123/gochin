package main

import (
	"fmt"
	"log"

	"github.com/gochin/framework/pkg/config"
)

// This file demonstrates how to use the Gochin configuration system
// and can be run independently to test configuration loading

func main() {
	fmt.Println("=== Gochin Framework Configuration ===")

	// Load the configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Display current configuration
	fmt.Printf("App Name: %s\n", cfg.App.Name)
	fmt.Printf("Environment: %s\n", cfg.App.Environment)
	fmt.Printf("Debug Mode: %t\n", cfg.App.Debug)
	fmt.Printf("Server Host: %s\n", cfg.Server.Host)
	fmt.Printf("Server Port: %d\n", cfg.Server.Port)
	fmt.Printf("Server Address: %s\n", config.GetServerAddr())

	fmt.Println("\n=== Environment Variables ===")
	fmt.Println("You can override these settings using environment variables:")
	fmt.Println("- GOCHIN_HOST: Server host (default: localhost)")
	fmt.Println("- GOCHIN_PORT: Server port (default: 8080)")
	fmt.Println("- GOCHIN_APP_NAME: Application name (default: Gochin API)")
	fmt.Println("- GOCHIN_ENV: Environment (default: development)")
	fmt.Println("- GOCHIN_DEBUG: Debug mode (default: true)")
	
	fmt.Println("\nExample:")
	fmt.Println("GOCHIN_HOST=0.0.0.0 GOCHIN_PORT=3000 go run config/main.go")
}