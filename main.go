package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

var (
	clients       = make(map[*websocket.Conn]bool)          // Map to keep track of connected clients
	broadcast     = make(chan *spotify.CurrentlyPlaying)    // Channel for broadcasting currently playing track
	upgrader      = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	spotifyClient spotify.Client                             // Spotify API client
	tokenSource   oauth2.TokenSource                         // OAuth2 token source
	config        *oauth2.Config                             // OAuth2 configuration
)

func main() {
	// Load environment variables from .env file if not already set
	if os.Getenv("CLIENT_ID") == "" || os.Getenv("CLIENT_SECRET") == "" || os.Getenv("REFRESH_TOKEN") == "" {
		if err := godotenv.Load(); err != nil {
			log.Fatalf("Error loading .env file: %v", err)
		}
	}

	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	refreshToken := os.Getenv("REFRESH_TOKEN")

	// Setup OAuth2 configuration for Spotify API
	config = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
	}

	token := &oauth2.Token{RefreshToken: refreshToken}
	tokenSource = config.TokenSource(context.Background(), token)

	// Create an OAuth2 HTTP client
	httpClient := oauth2.NewClient(context.Background(), tokenSource)
	spotifyClient = spotify.NewClient(httpClient)

	// Handle WebSocket connections at the root endpoint
	http.HandleFunc("/", ConnectionHandler)

	// Log server start-up and initialize background tasks
	log.Println("Server started on :3000")
	go TrackFetcher()   // Periodically fetch currently playing track from Spotify
	go MessageHandler() // Broadcast messages to connected clients

	// Start the HTTP server
	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

// ConnectionHandler upgrades HTTP connections to WebSocket and handles communication with clients
func ConnectionHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	clients[ws] = true

	// Immediately send the current track to the newly connected client
	currentTrack, err := spotifyClient.PlayerCurrentlyPlaying()
	if err != nil {
		log.Printf("Error getting currently playing track: %v", err)
		ws.Close()
		delete(clients, ws)
		return
	}

	// Send the current track information to the client
	err = ws.WriteJSON(currentTrack)
	if err != nil {
		ws.Close()
		delete(clients, ws)
		return
	}

	// Keep the connection open to listen for incoming messages (heartbeat)
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			delete(clients, ws)
			break
		}
	}
}

// MessageHandler continuously listens for messages on the broadcast channel and sends them to all connected clients
func MessageHandler() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}

// TrackFetcher periodically fetches the currently playing track from the Spotify API and broadcasts it to clients
func TrackFetcher() {
	for {
		// Fetch the currently playing track
		current, err := spotifyClient.PlayerCurrentlyPlaying()
		if err != nil {
			log.Printf("Error getting currently playing track: %v", err)
			// Refresh the access token if it has expired
			if err.Error() == "token expired" {
				log.Println("Token expired, refreshing token...")
				newToken, err := tokenSource.Token()
				if err != nil {
					log.Fatalf("Couldn't refresh token: %v", err)
				}
				httpClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(newToken))
				spotifyClient = spotify.NewClient(httpClient)
			}
			// Wait before retrying to avoid overwhelming the API
			time.Sleep(30 * time.Minute)
			continue
		}
		// Broadcast the current track information to all clients
		broadcast <- current
		// Fetch track information every 5 seconds
		time.Sleep(5 * time.Second)
	}
}
