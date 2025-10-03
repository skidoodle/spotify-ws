package websocket

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"spotify-ws/internal/spotify"
)

// Server is the main application orchestrator.
type Server struct {
	addr          string
	httpServer    *http.Server
	hub           *Hub
	poller        *Poller
	originChecker func(string) bool
}

// NewServer creates a new, fully configured WebSocket server.
func NewServer(addr string, allowedOrigins []string, spotifyClient *spotify.Client, realtime bool) *Server {
	hub := NewHub()
	poller := NewPoller(spotifyClient, hub, realtime)

	originChecker := func(origin string) bool {
		if len(allowedOrigins) == 0 {
			return true
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

// Run starts the server and its components.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	wsHandler := s.newWebsocketHandler()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		isWebSocket := r.Header.Get("Upgrade") == "websocket" &&
			strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")

		if isWebSocket {
			wsHandler.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Upgrade", "websocket")
		w.Header().Set("Connection", "Upgrade")
		w.Header().Set("X-Source", "github.com/skidoodle/spotify-ws")
		w.WriteHeader(http.StatusUpgradeRequired)
		if _, err := w.Write([]byte("426 Upgrade Required (github.com/skidoodle/spotify-ws)")); err != nil {
			slog.Warn("failed to write upgrade required response", "error", err)
		}
	})

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		s.hub.Run(ctx)
	}()

	go func() {
		defer wg.Done()
		s.poller.Run(ctx)
	}()

	go func() {
		<-ctx.Done()
		slog.Info("shutdown signal received, stopping http server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("http server shutdown error", "error", err)
		}
	}()

	slog.Info("http server listening", "addr", s.addr)
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	wg.Wait()

	return nil
}
