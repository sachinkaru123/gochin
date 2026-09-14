package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	// Load environment variables from a .env file if present
	"github.com/joho/godotenv"
)

// AppConfig holds the main application configuration
type AppConfig struct {
	Server   ServerConfig    `json:"server"`
	App      AppSettings     `json:"app"`
	Auth     AuthConfig      `json:"auth"`
	Log      LogConfig       `json:"log"`
	Storage  StorageConfig   `json:"storage"`
	Mail     MailConfig      `json:"mail"`
	Database *DatabaseConfig `json:"database"`
}

// LogConfig controls application logging.
type LogConfig struct {
	Dir      string `json:"dir"`
	Level    string `json:"level"`
	Format   string `json:"format"`
	ToFile   bool   `json:"to_file"`
	ToStdout bool   `json:"to_stdout"`
}

// StorageConfig controls file storage and static serving.
type StorageConfig struct {
	// Root is where uploaded files live. It is deliberately not served
	// statically, so an uploaded file can never be fetched back same-origin.
	Root string `json:"root"`
	// PublicDir is served at PublicURL.
	PublicDir string `json:"public_dir"`
	PublicURL string `json:"public_url"`
	// MaxUploadBytes caps a single uploaded file.
	MaxUploadBytes int64 `json:"max_upload_bytes"`
}

// MailConfig controls outbound email.
type MailConfig struct {
	// Driver is log, smtp or array. It defaults to log so that a
	// misconfigured development machine cannot email real people.
	Driver      string `json:"driver"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"-"`
	Password    string `json:"-"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
	TLS         bool   `json:"tls"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	// TokenTTL is how long an issued token stays valid; 0 means forever.
	TokenTTL time.Duration `json:"token_ttl"`
	// IdleTimeout expires tokens unused for this long; 0 disables it.
	IdleTimeout time.Duration `json:"idle_timeout"`
	// LoginRatePerMinute is the per-IP budget on credential endpoints. The
	// global limiter defaults to off, so login needs its own.
	LoginRatePerMinute int `json:"login_rate_per_minute"`

	Argon2MemoryKiB     int `json:"argon2_memory_kib"`
	Argon2Time          int `json:"argon2_time"`
	Argon2Parallelism   int `json:"argon2_parallelism"`
	Argon2MaxConcurrent int `json:"argon2_max_concurrent"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`

	// APIPrefix is prepended to every route registered through the router,
	// except those registered on the root group (/health, /metrics, /).
	APIPrefix string `json:"api_prefix"`

	// MaxBodyBytes caps JSON request bodies decoded by Context.Bind.
	MaxBodyBytes int64 `json:"max_body_bytes"`

	// HandlerTimeout bounds a single request; it cancels the request context
	// so in-flight database queries are cancelled too.
	HandlerTimeout time.Duration `json:"handler_timeout"`

	// TrustProxy enables X-Forwarded-For parsing. Only enable it behind a
	// proxy that overwrites the header, or clients can spoof their address.
	TrustProxy bool `json:"trust_proxy"`

	// RateLimit is the per-client request budget; 0 disables limiting.
	RateLimitPerMinute int `json:"rate_limit_per_minute"`

	// CORSAllowedOrigins is a comma-separated list, or "*" for any origin.
	CORSAllowedOrigins string `json:"cors_allowed_origins"`
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

	env := getEnvString("GOCHIN_ENV", "development")

	config = &AppConfig{
		Server: ServerConfig{
			Host:               getEnvString("GOCHIN_HOST", "localhost"),
			Port:               getEnvInt("GOCHIN_PORT", 8080),
			APIPrefix:          getEnvString("GOCHIN_API_PREFIX", "/api"),
			MaxBodyBytes:       int64(getEnvInt("GOCHIN_MAX_BODY_BYTES", 1<<20)),
			HandlerTimeout:     time.Duration(getEnvInt("GOCHIN_HANDLER_TIMEOUT_SECONDS", 15)) * time.Second,
			TrustProxy:         getEnvBool("GOCHIN_TRUST_PROXY", false),
			RateLimitPerMinute: getEnvInt("GOCHIN_RATE_LIMIT_PER_MINUTE", 0),
			CORSAllowedOrigins: getEnvString("GOCHIN_CORS_ORIGINS", ""),
		},
		App: AppSettings{
			Name:        getEnvString("GOCHIN_APP_NAME", "Gochin API"),
			Environment: env,
			// Debug serializes internal error text to clients, so it must
			// never default to on in production.
			Debug: getEnvBool("GOCHIN_DEBUG", env != "production"),
		},
		Auth: AuthConfig{
			TokenTTL:            time.Duration(getEnvInt("GOCHIN_AUTH_TOKEN_TTL_HOURS", 720)) * time.Hour,
			IdleTimeout:         time.Duration(getEnvInt("GOCHIN_AUTH_IDLE_TIMEOUT_HOURS", 0)) * time.Hour,
			LoginRatePerMinute:  getEnvInt("GOCHIN_LOGIN_RATE_PER_MINUTE", 5),
			Argon2MemoryKiB:     getEnvInt("GOCHIN_ARGON2_MEMORY_KIB", 19456),
			Argon2Time:          getEnvInt("GOCHIN_ARGON2_TIME", 2),
			Argon2Parallelism:   getEnvInt("GOCHIN_ARGON2_PARALLELISM", 1),
			Argon2MaxConcurrent: getEnvInt("GOCHIN_ARGON2_MAX_CONCURRENT", 4),
		},
		Log: LogConfig{
			Dir:      getEnvString("GOCHIN_LOG_DIR", "storage/logs"),
			Level:    getEnvString("GOCHIN_LOG_LEVEL", defaultLogLevel(env)),
			Format:   getEnvString("GOCHIN_LOG_FORMAT", defaultLogFormat(env)),
			ToFile:   getEnvBool("GOCHIN_LOG_TO_FILE", true),
			ToStdout: getEnvBool("GOCHIN_LOG_TO_STDOUT", true),
		},
		Storage: StorageConfig{
			Root:           getEnvString("GOCHIN_STORAGE_ROOT", "storage/app"),
			PublicDir:      getEnvString("GOCHIN_PUBLIC_DIR", "public"),
			PublicURL:      getEnvString("GOCHIN_PUBLIC_URL", "/assets"),
			MaxUploadBytes: int64(getEnvInt("GOCHIN_MAX_UPLOAD_BYTES", 10<<20)),
		},
		Mail: MailConfig{
			Driver:      getEnvString("GOCHIN_MAIL_DRIVER", "log"),
			Host:        getEnvString("GOCHIN_MAIL_HOST", "localhost"),
			Port:        getEnvInt("GOCHIN_MAIL_PORT", 1025),
			Username:    getEnvString("GOCHIN_MAIL_USERNAME", ""),
			Password:    getEnvString("GOCHIN_MAIL_PASSWORD", ""),
			FromAddress: getEnvString("GOCHIN_MAIL_FROM_ADDRESS", "no-reply@gochin.test"),
			FromName:    getEnvString("GOCHIN_MAIL_FROM_NAME", "Gochin"),
			TLS:         getEnvBool("GOCHIN_MAIL_TLS", true),
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

// defaultLogLevel is verbose in development and quiet in production, where
// debug volume costs money and buries the lines that matter.
func defaultLogLevel(env string) string {
	if env == "production" {
		return "info"
	}
	return "debug"
}

// defaultLogFormat is machine-readable in production for log aggregators, and
// human-readable elsewhere.
func defaultLogFormat(env string) string {
	if env == "production" {
		return "json"
	}
	return "text"
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
