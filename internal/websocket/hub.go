package websocket

import (
	"context"
	"log/slog"
	"spotify-ws/internal/spotify"
	"sync"

	"golang.org/x/net/websocket"
)

// Hub manages the set of active clients and broadcasts messages.
type Hub struct {
	clients    map[*websocket.Conn]struct{}
	mu         sync.RWMutex
	realtime   bool
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	broadcast  chan *spotify.CurrentlyPlaying
}

// NewHub creates a new Hub.
func NewHub(realtime bool) *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]struct{}),
		realtime:   realtime,
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		broadcast:  make(chan *spotify.CurrentlyPlaying),
	}
}

// Run starts the hub's event loop. It must be run in a separate goroutine.
func (h *Hub) Run(ctx context.Context) {
	slog.Info("hub started")
	defer slog.Info("hub stopped")

	for {
		select {
		case <-ctx.Done():
			h.closeAllConnections()
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
			slog.Debug("client registered", "remoteAddr", client.RemoteAddr())
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
			}
			h.mu.Unlock()
			slog.Debug("client unregistered", "remoteAddr", client.RemoteAddr())
		case state := <-h.broadcast:
			h.broadcastState(state)
		}
	}
}

// Broadcast sends a state update to all connected clients.
func (h *Hub) Broadcast(state *spotify.CurrentlyPlaying) {
	h.broadcast <- state
}

// broadcastState handles the actual message sending.
func (h *Hub) broadcastState(state *spotify.CurrentlyPlaying) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.clients) == 0 {
		return
	}

	clientPayload := newPlaybackState(state, h.realtime)
	for client := range h.clients {
		go func(c *websocket.Conn) {
			if err := websocket.JSON.Send(c, clientPayload); err != nil {
				slog.Warn("failed to broadcast message", "error", err, "remoteAddr", c.RemoteAddr())
			}
		}(client)
	}
}

// closeAllConnections closes all active client connections during shutdown.
func (h *Hub) closeAllConnections() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		if err := client.Close(); err != nil {
			slog.Warn("error closing client connection during shutdown", "error", err, "remoteAddr", client.RemoteAddr())
		}
	}
}
