# Database Configuration & Usage

## Overview
Your Gochin framework now includes comprehensive PostgreSQL database support with automatic connection testing during server startup.

## Configuration

### Environment Variables (`.env` file):
```env
GOCHIN_DB_HOST=localhost
GOCHIN_DB_PORT=5432
GOCHIN_DB_USER=postgres
GOCHIN_DB_PASSWORD=123
GOCHIN_DB_NAME=chingo_db

# Optional settings (with defaults):
GOCHIN_DB_SSL_MODE=disable
GOCHIN_DB_MAX_OPEN_CONNS=25
GOCHIN_DB_MAX_IDLE_CONNS=5
GOCHIN_DB_MAX_LIFETIME=5  # minutes
```

## Server Startup Integration

When you run `gochin run start`, the server now automatically:
1. ✅ Tests database connection
2. 🚀 Starts HTTP server
3. ⚠️  Shows warnings if database is unavailable (but continues running)

### Command Options:
```bash
# Normal startup with database test
gochin run start

# Skip database test (faster startup)
gochin run start --skip-db-test

# Custom host/port with database test
gochin run start --host 0.0.0.0 --port 3000
```

## Database Usage in Your Application

### 1. Get Database Connection (Singleton Pattern)
```go
import "github.com/gochin/framework/pkg/database"

// Get the shared connection pool
db, err := database.GetConnection()
if err != nil {
    log.Fatal(err)
}

// Use db for queries...
```

### 2. Check Database Health
```go
// Simple ping test
if database.IsConnected() {
    fmt.Println("Database is available")
}

// Or with error details
err := database.Ping()
if err != nil {
    log.Printf("Database issue: %v", err)
}
```

### 3. Example Queries
```go
// Simple query
var count int
err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)

// Insert with parameters
_, err = db.Exec("INSERT INTO users (name, email) VALUES ($1, $2)", 
    "John", "john@example.com")

// Query multiple rows
rows, err := db.Query("SELECT id, name FROM users WHERE active = $1", true)
defer rows.Close()
```

## Testing & Examples

### Test Database Connection
```bash
# Standalone database test
CGO_ENABLED=0 go run cmd/test-db/main.go

# Database usage examples
CGO_ENABLED=0 go run examples/database-usage/main.go
```

## Server Startup Output

When you run `gochin run start`, you'll see:
```
Starting Gochin server...
App: Gochin API (development)
Server starting on localhost:9081
Debug mode: true

🔌 Testing database connection...
✅ Database connection successful!

🚀 Gochin server is ready!
   ➜ Local:   http://localhost:9081
```

## Benefits

1. **Automatic Testing** - Database connectivity is verified on every startup
2. **Connection Pooling** - Efficient database connection management
3. **Graceful Handling** - Server starts even if database is temporarily unavailable
4. **Environment Configuration** - All settings via `.env` file
5. **Singleton Pattern** - Single connection pool shared across your application
6. **PostgreSQL Optimized** - Tuned for PostgreSQL best practices

## Troubleshooting

If database connection fails:
1. Ensure PostgreSQL is running: `sudo service postgresql start`
2. Create the database: `createdb chingo_db`
3. Check user permissions
4. Verify `.env` settings
5. Use `--skip-db-test` flag for server startup without database

## Next Steps

- Create models and migrations
- Add database middleware for HTTP handlers  
- Implement user authentication with database storage
- Add database health check endpoints