package database

import (
	"database/sql"
	"log"
	"sync"

	"github.com/gochin/framework/pkg/config"
)

var (
	// Global database connection instance
	dbInstance *sql.DB
	// Mutex to ensure thread-safe initialization
	dbOnce sync.Once
	// Error from initialization
	dbInitError error
)

// GetConnection returns a singleton database connection
// This ensures we only create one connection pool per application instance
func GetConnection() (*sql.DB, error) {
	dbOnce.Do(func() {
		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			dbInitError = err
			return
		}

		// Connect to database
		dbInstance, dbInitError = cfg.Database.Connect()
		if dbInitError != nil {
			log.Printf("Failed to initialize database connection: %v", dbInitError)
			return
		}

		log.Println("Database connection pool initialized")
	})

	return dbInstance, dbInitError
}

// CloseConnection closes the global database connection
func CloseConnection() {
	if dbInstance != nil {
		if err := dbInstance.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		} else {
			log.Println("Database connection closed")
		}
	}
}

// Ping checks if the database connection is alive
func Ping() error {
	db, err := GetConnection()
	if err != nil {
		return err
	}
	return db.Ping()
}

// IsConnected returns true if database is connected and responsive
func IsConnected() bool {
	return Ping() == nil
}
