package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"spotify-ws/internal/spotify"
)

// Poller is responsible for fetching data from the Spotify API periodically.
type Poller struct {
	client    *spotify.Client
	hub       *Hub
	realtime  bool
	lastState *spotify.CurrentlyPlaying
	mu        sync.RWMutex
}

// NewPoller creates a new Poller.
func NewPoller(client *spotify.Client, hub *Hub, realtime bool) *Poller {
	return &Poller{
		client:   client,
		hub:      hub,
		realtime: realtime,
	}
}

// Run starts the polling loop.
func (p *Poller) Run(ctx context.Context) {
	slog.Info("poller started")
	defer slog.Info("poller stopped")

	p.UpdateState(ctx)

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
		if !p.realtime {
			trackName := "Nothing"
			if current.Item != nil {
				trackName = current.Item.Name
			}
			slog.Info("state changed, broadcasting update", "isPlaying", current.IsPlaying, "track", trackName)
		}

		payload := newPlaybackState(current, p.realtime)
		message, err := json.Marshal(payload)
		if err != nil {
			slog.Error("failed to marshal playback state", "error", err)
			return
		}

		p.hub.broadcast <- message
	}
}

// hasStateChanged performs a robust comparison between the new and old states.
func (p *Poller) hasStateChanged(current *spotify.CurrentlyPlaying) bool {
	if p.lastState == nil {
		return true
	}
	if p.realtime && current.IsPlaying && current.Item != nil {
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
