package main

import (
	"fmt"
	"log"

	"github.com/gochin/framework/pkg/database"
)

func main() {
	fmt.Println("🔗 Database Connection Example")
	fmt.Println("==============================")
	
	// Get database connection (singleton pattern)
	db, err := database.GetConnection()
	if err != nil {
		log.Fatalf("Failed to get database connection: %v", err)
	}

	// Example 1: Simple query
	fmt.Println("\n📊 Executing sample queries:")
	
	// Query current time from database
	var currentTime string
	err = db.QueryRow("SELECT NOW()").Scan(&currentTime)
	if err != nil {
		log.Printf("Error querying current time: %v", err)
	} else {
		fmt.Printf("Database time: %s\n", currentTime)
	}

	// Query database version
	var version string
	err = db.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		log.Printf("Error querying version: %v", err)
	} else {
		fmt.Printf("Database version: %s\n", version[:50])
	}

	// Example 2: Check if connection is alive
	fmt.Println("\n🏥 Connection health check:")
	if database.IsConnected() {
		fmt.Println("✅ Database is connected and responsive")
	} else {
		fmt.Println("❌ Database connection is not working")
	}

	// Example 3: Create a sample table (if it doesn't exist)
	fmt.Println("\n📋 Creating sample table:")
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS sample_users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(100) UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Printf("Error creating table: %v", err)
	} else {
		fmt.Println("✅ Sample table 'sample_users' ensured")
	}

	// Example 4: Insert sample data
	fmt.Println("\n📝 Inserting sample data:")
	insertSQL := `
	INSERT INTO sample_users (name, email) 
	VALUES ($1, $2) 
	ON CONFLICT (email) DO NOTHING 
	RETURNING id`
	
	var userID int
	err = db.QueryRow(insertSQL, "John Doe", "john@example.com").Scan(&userID)
	if err != nil {
		fmt.Printf("User might already exist or error occurred: %v\n", err)
	} else {
		fmt.Printf("✅ Inserted user with ID: %d\n", userID)
	}

	// Example 5: Query data
	fmt.Println("\n🔍 Querying users:")
	rows, err := db.Query("SELECT id, name, email, created_at FROM sample_users LIMIT 5")
	if err != nil {
		log.Printf("Error querying users: %v", err)
	} else {
		defer rows.Close()
		
		fmt.Println("Users in database:")
		for rows.Next() {
			var id int
			var name, email, createdAt string
			
			err = rows.Scan(&id, &name, &email, &createdAt)
			if err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}
			
			fmt.Printf("  - ID: %d, Name: %s, Email: %s\n", id, name, email)
		}
	}

	fmt.Println("\n🎉 Database example completed successfully!")
	fmt.Println("\nNOTE: The database connection will be reused across your application")
	fmt.Println("Use database.GetConnection() anywhere in your code to get the same connection pool")
}