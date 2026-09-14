// Package logs is the application logging facade.
//
// Call it from anywhere:
//
//	logs.Debug("checkout failed", "order_id", id)
//	logs.Dump(someVariable)
//	logs.Channel("payments").Info("captured", "amount", amount)
//
// Init installs the configured handler as slog's default, so every package
// that already logs through slog — the router middleware, the auth guard, the
// HTTP server's error log — writes to the same destination automatically.
package logs

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Config controls where and how logs are written.
type Config struct {
	Dir      string // directory for log files
	Level    string // debug, info, warn, error
	Format   string // text or json
	ToFile   bool
	ToStdout bool
}

// DefaultConfig returns development-friendly settings.
func DefaultConfig() Config {
	return Config{
		Dir:      "storage/logs",
		Level:    "debug",
		Format:   "text",
		ToFile:   true,
		ToStdout: true,
	}
}

var (
	mu       sync.RWMutex
	cfg      = DefaultConfig()
	base     = slog.Default()
	channels = map[string]*slog.Logger{}
	writers  []*dailyWriter
)

// Init configures logging and installs it as slog's default.
func Init(c Config) error {
	if c.Dir == "" {
		c.Dir = "storage/logs"
	}
	if c.Level == "" {
		c.Level = "info"
	}
	if c.Format == "" {
		c.Format = "text"
	}

	logger, w, err := build(c, "gochin")
	if err != nil {
		return err
	}

	mu.Lock()
	cfg = c
	base = logger
	channels = map[string]*slog.Logger{}
	if w != nil {
		writers = append(writers, w)
	}
	mu.Unlock()

	// Everything already logging through slog now lands here too.
	slog.SetDefault(logger)
	return nil
}

// build assembles a logger writing to a per-channel file and/or stdout.
func build(c Config, prefix string) (*slog.Logger, *dailyWriter, error) {
	var (
		sinks  []interface{ Write([]byte) (int, error) }
		writer *dailyWriter
	)

	if c.ToFile {
		w, err := newDailyWriter(c.Dir, prefix)
		if err != nil {
			return nil, nil, err
		}
		writer = w
		sinks = append(sinks, w)
	}
	if c.ToStdout || len(sinks) == 0 {
		sinks = append(sinks, os.Stdout)
	}

	var out io.Writer = multiWriter{writers: sinks}
	opts := &slog.HandlerOptions{Level: parseLevel(c.Level)}

	var handler slog.Handler
	if strings.EqualFold(c.Format, "json") {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}

	return slog.New(handler), writer, nil
}

func parseLevel(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Logger returns the default application logger.
func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return base
}

// Channel returns a logger writing to its own file, e.g.
// storage/logs/payments-2026-09-14.log.
//
// Loggers are cached, so calling this in a hot path does not reopen files.
func Channel(name string) *slog.Logger {
	mu.RLock()
	if l, ok := channels[name]; ok {
		mu.RUnlock()
		return l
	}
	current := cfg
	mu.RUnlock()

	logger, w, err := build(current, name)
	if err != nil {
		// A bad channel must not take the process down; fall back to default.
		Logger().Error("creating log channel", "channel", name, "error", err)
		return Logger()
	}
	logger = logger.With("channel", name)

	mu.Lock()
	defer mu.Unlock()
	if existing, ok := channels[name]; ok {
		// Another goroutine won the race; discard ours.
		_ = w.Close()
		return existing
	}
	channels[name] = logger
	if w != nil {
		writers = append(writers, w)
	}
	return logger
}

// With returns a logger carrying the given attributes.
func With(args ...any) *slog.Logger { return Logger().With(args...) }

// Close releases every open log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	for _, w := range writers {
		_ = w.Close()
	}
	writers = nil
}

// Path returns the file the default logger is writing to, or "" when file
// logging is disabled.
func Path() string {
	mu.RLock()
	defer mu.RUnlock()
	if len(writers) == 0 {
		return ""
	}
	return writers[0].Path()
}

func Debug(msg string, args ...any) { Logger().Debug(msg, args...) }
func Info(msg string, args ...any)  { Logger().Info(msg, args...) }
func Warn(msg string, args ...any)  { Logger().Warn(msg, args...) }
func Error(msg string, args ...any) { Logger().Error(msg, args...) }

func DebugContext(ctx context.Context, msg string, args ...any) {
	Logger().DebugContext(ctx, msg, args...)
}
func InfoContext(ctx context.Context, msg string, args ...any) {
	Logger().InfoContext(ctx, msg, args...)
}
func ErrorContext(ctx context.Context, msg string, args ...any) {
	Logger().ErrorContext(ctx, msg, args...)
}
