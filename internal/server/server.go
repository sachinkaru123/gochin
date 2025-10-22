package server

import (
	"fmt"
	"net/http"
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
	"log"
)

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

// Start starts the HTTP server with graceful shutdown
func (s *Server) Start() error {
	// Create a simple mux router
	mux := http.NewServeMux()
	
	// Add some basic routes
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/health", s.handleHealth)
	
	// Create the server
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.host, s.port),
		Handler: mux,
	}
	
	// Channel to listen for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	
	// Start server in a goroutine
	go func() {
		fmt.Printf("🚀 Server starting on http://%s:%d\n", s.host, s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()
	
	// Wait for interrupt signal
	<-stop
	fmt.Println("\n🛑 Shutting down server...")
	
	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}
	
	fmt.Println("✅ Server stopped gracefully")
	return nil
}

// handleHome handles the home route
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Gochin Framework</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 600px; margin: 0 auto; text-align: center; }
        .logo { font-size: 48px; color: #2563eb; margin-bottom: 20px; }
        .subtitle { color: #64748b; margin-bottom: 30px; }
        .links { margin-top: 30px; }
        .links a { color: #2563eb; text-decoration: none; margin: 0 10px; }
        .links a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">🚀 Gochin</div>
        <h1>Welcome to Gochin Framework!</h1>
        <p class="subtitle">Your Go web framework is running successfully</p>
        <div class="links">
            <a href="/health">Health Check</a>
        </div>
    </div>
</body>
</html>
`)
}

// handleHealth handles the health check route
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`, time.Now().UTC().Format(time.RFC3339))
}