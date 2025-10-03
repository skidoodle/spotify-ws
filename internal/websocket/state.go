package websocket

import "spotify-ws/internal/spotify"

// PlaybackState is the client-facing data structure.
type PlaybackState struct {
	IsPlaying  bool               `json:"is_playing"`
	ProgressMs int                `json:"progress_ms,omitempty"`
	Timestamp  int64              `json:"timestamp,omitempty"`
	Item       *spotify.TrackItem `json:"item"`
}

// newPlaybackState creates a client-facing PlaybackState from the internal Spotify data.
func newPlaybackState(data *spotify.CurrentlyPlaying, realtime bool) PlaybackState {
	if data == nil {
		return PlaybackState{IsPlaying: false}
	}
	state := PlaybackState{
		IsPlaying: data.IsPlaying,
		Item:      data.Item,
	}
	if realtime {
		state.ProgressMs = data.ProgressMs
		state.Timestamp = data.Timestamp
	}
	return state
}
