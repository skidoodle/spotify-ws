package websocket

import (
	"context"
	"log/slog"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	clients    map[*Client]struct{}
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client

	// The last known state, protected by a mutex. This is sent to new clients.
	lastState []byte
	mu        sync.RWMutex
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's event loop. It must be run in a separate goroutine.
func (h *Hub) Run(ctx context.Context) {
	slog.Info("hub started")
	defer slog.Info("hub stopped")

	for {
		select {
		case <-ctx.Done():
			for client := range h.clients {
				close(client.send)
			}
			return
		case client := <-h.register:
			h.clients[client] = struct{}{}
			h.mu.RLock()
			if h.lastState != nil {
				client.send <- h.lastState
			}
			h.mu.RUnlock()
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			h.mu.Lock()
			h.lastState = message
			h.mu.Unlock()

			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					slog.Warn("client send buffer full, disconnecting", "remoteAddr", client.conn.RemoteAddr())
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
