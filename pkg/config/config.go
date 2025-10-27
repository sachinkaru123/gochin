package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
    
	// Load environment variables from a .env file if present
	"github.com/joho/godotenv"
)

// AppConfig holds the main application configuration
type AppConfig struct {
	Server   ServerConfig     `json:"server"`
	App      AppSettings      `json:"app"`
	Database *DatabaseConfig  `json:"database"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// AppSettings holds general application settings
type AppSettings struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
	Debug       bool   `json:"debug"`
}

var config *AppConfig

// Load initializes and loads the application configuration
func Load() (*AppConfig, error) {
	if config != nil {
		return config, nil
	}

	// Attempt to load variables from .env in the project root. If the file is
	// missing, ignore the error and continue using the OS environment.
	_ = godotenv.Load()

	config = &AppConfig{
		Server: ServerConfig{
			Host: getEnvString("GOCHIN_HOST", "localhost"),
			Port: getEnvInt("GOCHIN_PORT", 8080),
		},
		App: AppSettings{
			Name:        getEnvString("GOCHIN_APP_NAME", "Gochin API"),
			Environment: getEnvString("GOCHIN_ENV", "development"),
			Debug:       getEnvBool("GOCHIN_DEBUG", true),
		},
		Database: LoadDatabaseConfig(),
	}

	return config, nil
}

// Get returns the loaded configuration instance
func Get() *AppConfig {
	if config == nil {
		// Try to load if not already loaded
		Load()
	}
	return config
}

// GetServerAddr returns the complete server address (host:port)
func GetServerAddr() string {
	cfg := Get()
	return fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
}

// getEnvString gets a string environment variable with a default fallback
func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvInt gets an integer environment variable with a default fallback
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

// getEnvBool gets a boolean environment variable with a default fallback
func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		val = strings.ToLower(val)
		return val == "true" || val == "1" || val == "yes"
	}
	return defaultVal
}

// SetHost updates the server host configuration
func SetHost(host string) {
	cfg := Get()
	cfg.Server.Host = host
}

// SetPort updates the server port configuration
func SetPort(port int) {
	cfg := Get()
	cfg.Server.Port = port
}