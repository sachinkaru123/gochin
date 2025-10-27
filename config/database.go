package config

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	Name         string `json:"name"`
	SSLMode      string `json:"ssl_mode"`
	MaxOpenConns int    `json:"max_open_conns"`
	MaxIdleConns int    `json:"max_idle_conns"`
	MaxLifetime  int    `json:"max_lifetime"` // in minutes
}

// LoadDatabaseConfig loads database configuration from environment variables
func LoadDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Host:         getEnvString("GOCHIN_DB_HOST", "localhost"),
		Port:         getEnvInt("GOCHIN_DB_PORT", 5432),
		User:         getEnvString("GOCHIN_DB_USER", "postgres"),
		Password:     getEnvString("GOCHIN_DB_PASSWORD", ""),
		Name:         getEnvString("GOCHIN_DB_NAME", "gochin_db"),
		SSLMode:      getEnvString("GOCHIN_DB_SSL_MODE", "disable"),
		MaxOpenConns: getEnvInt("GOCHIN_DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns: getEnvInt("GOCHIN_DB_MAX_IDLE_CONNS", 5),
		MaxLifetime:  getEnvInt("GOCHIN_DB_MAX_LIFETIME", 5),
	}
}

// GetConnectionString returns the PostgreSQL connection string
func (db *DatabaseConfig) GetConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		db.Host, db.Port, db.User, db.Password, db.Name, db.SSLMode,
	)
}

// Connect establishes a database connection
func (db *DatabaseConfig) Connect() (*sql.DB, error) {
	connStr := db.GetConnectionString()
	
	log.Printf("Connecting to database: host=%s port=%d dbname=%s user=%s", 
		db.Host, db.Port, db.Name, db.User)
	
	connection, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	
	// Configure connection pool
	connection.SetMaxOpenConns(db.MaxOpenConns)
	connection.SetMaxIdleConns(db.MaxIdleConns)
	connection.SetConnMaxLifetime(time.Duration(db.MaxLifetime) * time.Minute)
	
	// Test the connection
	if err := connection.Ping(); err != nil {
		connection.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	log.Println("✅ Database connection established successfully")
	return connection, nil
}

// TestConnection tests the database connection without keeping it open
func (db *DatabaseConfig) TestConnection() error {
	conn, err := db.Connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	
	// Test with a simple query
	var version string
	err = conn.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to query database version: %w", err)
	}
	
	log.Printf("Database version: %s", version)
	return nil
}

// Utility functions (these should match the ones in pkg/config/config.go)
func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}