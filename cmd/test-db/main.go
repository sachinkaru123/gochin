package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gochin/framework/pkg/config"
)

// This is a simple test program to verify your database configuration
func main() {
	fmt.Println("🔍 Testing Gochin Database Configuration")
	fmt.Println("====================================================")
	
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	// Display database configuration
	db := cfg.Database
	fmt.Printf("Database Host: %s\n", db.Host)
	fmt.Printf("Database Port: %d\n", db.Port)
	fmt.Printf("Database Name: %s\n", db.Name)
	fmt.Printf("Database User: %s\n", db.User)
	fmt.Printf("SSL Mode: %s\n", db.SSLMode)
	fmt.Printf("Max Open Connections: %d\n", db.MaxOpenConns)
	fmt.Printf("Max Idle Connections: %d\n", db.MaxIdleConns)
	fmt.Printf("Connection Max Lifetime: %d minutes\n", db.MaxLifetime)
	
	fmt.Println("\n🔗 Connection String:")
	// Don't print the actual connection string as it contains the password
	fmt.Printf("postgres://%s:***@%s:%d/%s?sslmode=%s\n", 
		db.User, db.Host, db.Port, db.Name, db.SSLMode)
	
	fmt.Println("\n🧪 Testing database connection...")
	
	// Test connection
	err = db.TestConnection()
	if err != nil {
		fmt.Printf("❌ Database connection failed: %v\n", err)
		fmt.Println("\n💡 Troubleshooting tips:")
		fmt.Println("1. Make sure PostgreSQL is running on your system")
		fmt.Println("2. Verify the database exists: CREATE DATABASE chingo_db;")
		fmt.Println("3. Check user permissions")
		fmt.Println("4. Verify your .env file settings")
		os.Exit(1)
	}
	
	fmt.Println("✅ Database connection successful!")
	fmt.Println("\n🎉 Your database configuration is working correctly!")
}