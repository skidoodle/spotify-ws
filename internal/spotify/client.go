package spotify

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"
)

const (
	tokenURL            = "https://accounts.spotify.com/api/token"
	currentlyPlayingURL = "https://api.spotify.com/v1/me/player/currently-playing"
)

// Client is a thread-safe client for interacting with the Spotify API.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new Spotify API client using the refresh token flow.
// The returned client is safe for concurrent use.
func NewClient(ctx context.Context, clientID, clientSecret, refreshToken string) *Client {
	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenURL,
		},
	}

	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	// The TokenSource is concurrency-safe and handles token refreshes automatically.
	tokenSource := conf.TokenSource(ctx, token)

	return &Client{
		httpClient: oauth2.NewClient(ctx, tokenSource),
	}
}

// CurrentlyPlaying fetches the user's currently playing track from the Spotify API.
func (c *Client) CurrentlyPlaying(ctx context.Context) (*CurrentlyPlaying, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentlyPlayingURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("failed to close spotify api response body", "error", err)
		}
	}()

	// When nothing is playing, Spotify returns 204 No Content.
	// We normalize this to a consistent struct response for the caller.
	if resp.StatusCode == http.StatusNoContent {
		return &CurrentlyPlaying{IsPlaying: false, Item: nil}, nil
	}

	var currentlyPlaying CurrentlyPlaying
	if err := json.NewDecoder(resp.Body).Decode(&currentlyPlaying); err != nil {
		return nil, err
	}

	return &currentlyPlaying, nil
}
