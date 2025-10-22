# Gochin Framework

A modern, fast, and flexible Go-based API framework with a powerful CLI for rapid application development.

## Features

- 🚀 **CLI Kernel**: Powerful command-line interface built with Cobra
- ⚙️ **Configuration Management**: Environment-based configuration system
- 🏗️ **Code Generation**: Auto-generate controllers, models, and more
- 🗃️ **Database Operations**: Built-in migration and seeding system
- 📁 **Modular Architecture**: Clean, organized project structure

## Installation

1. Clone the repository:
```bash
git clone https://github.com/sachinkaru123/gochin-v1.git
cd framework
```

2. **Smart Installation** (automatically detects your OS):
```bash
make install
```

This will:
- Auto-detect your operating system using `runtime.GOOS`
- Build the binary for your platform
- Install to the appropriate system location:
  - **Linux/macOS**: `/usr/local/bin/gochin`
  - **Windows**: `C:\Program Files\Gochin\gochin.exe`
- Handle permissions automatically (uses `sudo` on Unix systems)

**Manual installation** (if needed):
```bash
make build                          # Build for current platform
make install-manual                 # Manual install (Linux/macOS only)
```

## CLI Commands

### Server Operations

Start the development server:
```bash
gochin run start
# or via Makefile
make start
```

Start with custom host and port:
```bash
gochin run start --host 0.0.0.0 --port 3000
```

### Code Generation

Generate a new controller:
```bash
gochin make controller UserController
# or simply
gochin make controller User
# or via Makefile
make controller name=User
```

### Database Operations

Run database migrations:
```bash
gochin db migrate
# or via Makefile
make migrate
```

Rollback last migration:
```bash
gochin db migrate --rollback
```

## Configuration

### Environment Variables

Gochin supports configuration through environment variables:

- `GOCHIN_HOST`: Server host (default: localhost)
- `GOCHIN_PORT`: Server port (default: 8080)
- `GOCHIN_APP_NAME`: Application name (default: Gochin API)
- `GOCHIN_ENV`: Environment (default: development)
- `GOCHIN_DEBUG`: Debug mode (default: true)

### Example Usage

```bash
# Start server with custom configuration
GOCHIN_HOST=0.0.0.0 GOCHIN_PORT=3000 gochin run start

# Test configuration loading
GOCHIN_APP_NAME="My API" go run config/main.go
```

## Project Structure

```
├── cmd/
│   └── gochin/           # CLI entrypoint
│       └── main.go
├── internal/
│   ├── cli/              # CLI kernel
│   │   ├── root.go       # Root command setup
│   │   └── command.go    # Base command interface
│   └── commands/         # Command implementations
│       ├── run.go        # Server operations
│       ├── make.go       # Code generation
│       └── db.go         # Database operations
├── pkg/
│   └── config/           # Configuration management
│       └── config.go
├── config/
│   └── main.go           # Configuration demo/test
├── app/
│   └── Controllers/      # Generated controllers
├── go.mod
└── README.md
```

## Cross-Platform Support

Gochin automatically detects your operating system and installs appropriately:

| OS | Architecture | Binary Name | Install Location | Requires Sudo |
|---|---|---|---|---|
| **Linux** | amd64, 386, arm64 | `gochin` | `/usr/local/bin/` | ✅ Yes |
| **macOS** | amd64, arm64 (M1/M2) | `gochin` | `/usr/local/bin/` | ✅ Yes |
| **Windows** | amd64, 386 | `gochin.exe` | `C:\Program Files\Gochin\` | ❌ No (uses UAC) |
| **FreeBSD** | amd64 | `gochin` | `/usr/local/bin/` | ✅ Yes |

**How it works:**
- Uses Go's `runtime.GOOS` and `runtime.GOARCH` for detection
- No manual OS configuration needed
- Single `make install` command works everywhere
- Smart permission handling per platform

**Demo OS detection:**
```bash
go run demo/os-detection.go
```

## Architecture

### CLI Kernel Design

- **Modular Commands**: Each command group (run, make, db) is self-contained
- **Auto-Registration**: Commands are automatically registered with the root CLI
- **Extensible**: Easy to add new commands and subcommands
- **Flag Support**: Rich flag and argument parsing with Cobra

### Command Registration System

Commands follow a consistent pattern:
1. Create command function returning `*cobra.Command`
2. Register in `internal/cli/root.go`
3. Implement business logic in the command handler

### Configuration System

- Environment variable support with fallback defaults
- Type-safe configuration loading
- Runtime configuration updates
- Centralized configuration management

## Development Status

This is the initial CLI foundation. The following features are implemented:

✅ Project structure and Go module setup  
✅ CLI kernel architecture with Cobra  
✅ Command registration system  
✅ Configuration system with host/port support  
✅ `gochin run start` command  
✅ `gochin make controller` framework  
✅ `gochin db migrate` framework  

### Next Steps

- HTTP server implementation for `run start`
- Controller template generation
- Database migration system
- Model and middleware generators
- Route management
- Testing framework integration

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/new-feature`
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

## License

MIT License - see LICENSE file for details.