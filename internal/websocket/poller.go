package websocket

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"spotify-ws/internal/spotify"

	"golang.org/x/net/websocket"
)

// Poller is responsible for fetching data from the Spotify API periodically.
type Poller struct {
	client    *spotify.Client
	hub       *Hub
	lastState *spotify.CurrentlyPlaying
	mu        sync.RWMutex
}

// NewPoller creates a new Poller.
func NewPoller(client *spotify.Client, hub *Hub) *Poller {
	return &Poller{
		client: client,
		hub:    hub,
	}
}

// Run starts the polling loop. It must be run in a separate goroutine.
func (p *Poller) Run(ctx context.Context) {
	slog.Info("poller started")
	defer slog.Info("poller stopped")

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.UpdateState(ctx)
		}
	}
}

// UpdateState fetches the latest state, compares it, and broadcasts if needed.
func (p *Poller) UpdateState(ctx context.Context) {
	current, err := p.client.CurrentlyPlaying(ctx)
	if err != nil {
		slog.Error("failed to get currently playing track", "error", err)
		return
	}

	p.mu.Lock()
	hasChanged := p.hasStateChanged(current)
	if hasChanged {
		p.lastState = current
	}
	p.mu.Unlock()

	if hasChanged {
		if !p.hub.realtime {
			trackName := "Nothing"
			if current.Item != nil {
				trackName = current.Item.Name
			}
			slog.Info("state changed, broadcasting update", "isPlaying", current.IsPlaying, "track", trackName)
		}
		p.hub.Broadcast(current)
	}
}

// SendLastState sends the cached state to a single new client.
func (p *Poller) SendLastState(ws *websocket.Conn) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.lastState == nil {
		return
	}
	clientPayload := newPlaybackState(p.lastState, p.hub.realtime)
	if err := websocket.JSON.Send(ws, clientPayload); err != nil {
		slog.Warn("failed to send initial state to client", "error", err, "remoteAddr", ws.RemoteAddr())
	}
}

// hasStateChanged performs a robust comparison between the new and old states.
// This function must be called within a lock.
func (p *Poller) hasStateChanged(current *spotify.CurrentlyPlaying) bool {
	if p.lastState == nil {
		return true
	}
	if p.hub.realtime && current.IsPlaying && current.Item != nil {
		return true
	}
	if p.lastState.IsPlaying != current.IsPlaying {
		return true
	}
	if (p.lastState.Item == nil) != (current.Item == nil) {
		return true
	}
	if p.lastState.Item != nil && current.Item != nil && p.lastState.Item.ID != current.Item.ID {
		return true
	}
	return false
}
