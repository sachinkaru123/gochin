package server

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gochin/framework/pkg/config"
	"github.com/gochin/framework/pkg/database"
)

const shutdownTimeout = 30 * time.Second

// Server represents the HTTP server
type Server struct {
	httpServer *http.Server
	host       string
	port       int
}

// New creates a new server instance
func New(host string, port int) *Server {
	return &Server{
		host: host,
		port: port,
	}
}

// Start builds the router and serves until interrupted.
func (s *Server) Start() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	handler, err := BuildRouter(cfg)
	if err != nil {
		return fmt.Errorf("failed to build router: %w", err)
	}

	// Cancelled on shutdown so in-flight database queries are cancelled too.
	baseCtx, cancelBase := context.WithCancel(context.Background())
	defer cancelBase()

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.host, s.port),
		Handler: handler,

		// Without ReadHeaderTimeout a client can hold a connection and a
		// goroutine open forever by never finishing its headers (Slowloris).
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// A hard backstop above the middleware timeout, which can still
		// render a clean 504.
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
		ErrorLog:       slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
		BaseContext:    func(net.Listener) context.Context { return baseCtx },
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("🚀 Server starting on http://%s:%d\n", s.host, s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	<-stop
	fmt.Println("\n🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		cancelBase()
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	cancelBase()
	database.CloseConnection()

	fmt.Println("✅ Server stopped gracefully")
	return nil
}
