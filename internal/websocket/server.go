package websocket

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"spotify-ws/internal/spotify"
)

// Server is the main application orchestrator. It owns all components
// and manages the application's lifecycle.
type Server struct {
	addr          string
	httpServer    *http.Server
	hub           *Hub
	poller        *Poller
	originChecker func(string) bool
}

// NewServer creates a new, fully configured WebSocket server.
func NewServer(addr string, allowedOrigins []string, spotifyClient *spotify.Client, realtime bool) *Server {
	hub := NewHub(realtime)
	poller := NewPoller(spotifyClient, hub)

	// Create a closure for origin checking to keep the Server's dependencies clean.
	originChecker := func(origin string) bool {
		if len(allowedOrigins) == 0 {
			return true // Allow all if not specified.
		}
		for _, allowedOrigin := range allowedOrigins {
			if allowedOrigin == origin {
				return true
			}
		}
		return false
	}

	return &Server{
		addr:          addr,
		hub:           hub,
		poller:        poller,
		originChecker: originChecker,
	}
}

// Run starts the server and its components. It blocks until the context is
// canceled and all components have shut down gracefully.
func (s *Server) Run(ctx context.Context) error {
	// Do an initial state fetch before starting the server.
	s.poller.UpdateState(ctx)

	mux := http.NewServeMux()
	mux.Handle("/", s.newWebsocketHandler())
	mux.HandleFunc("/health", healthHandler)

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	var wg sync.WaitGroup
	wg.Add(2) // For the hub and the poller.

	go func() {
		defer wg.Done()
		s.hub.Run(ctx)
	}()

	go func() {
		defer wg.Done()
		s.poller.Run(ctx)
	}()

	// Start the HTTP server.
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
		}
	}()

	// Wait for shutdown signal.
	<-ctx.Done()
	slog.Info("shutdown signal received")

	// The hub and poller will stop automatically via the context.
	// We just need to shut down the HTTP server and wait for goroutines to finish.
	s.shutdown()
	wg.Wait()

	return nil
}

// shutdown gracefully shuts down the HTTP server.
func (s *Server) shutdown() {
	slog.Info("shutting down http server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}
}
