package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

var (
	clients      = make(map[*websocket.Conn]bool)       // Map to keep track of connected clients
	clientsMutex sync.Mutex                             // Mutex to protect access to clients map
	broadcast    = make(chan *spotify.CurrentlyPlaying) // Channel for broadcasting currently playing track
	connect      = make(chan *websocket.Conn)           // Channel for managing new connections
	disconnect   = make(chan *websocket.Conn)           // Channel for managing client disconnections
	upgrader     = websocket.Upgrader{
		CheckOrigin:      func(r *http.Request) bool { return true }, // Allow all origins
		HandshakeTimeout: 10 * time.Second,                           // Timeout for WebSocket handshake
		ReadBufferSize:   1024,                                       // Buffer size for reading incoming messages
		WriteBufferSize:  1024,                                       // Buffer size for writing outgoing messages
		Subprotocols:     []string{"binary"},                         // Supported subprotocols
	}
	spotifyClient spotify.Client     // Spotify API client
	tokenSource   oauth2.TokenSource // OAuth2 token source
	config        *oauth2.Config     // OAuth2 configuration
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
	go TrackFetcher()      // Periodically fetch currently playing track from Spotify
	go MessageHandler()    // Broadcast messages to connected clients
	go ConnectionManager() // Manage client connections

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
	connect <- ws

	defer func() {
		disconnect <- ws
		err := ws.Close()
		if err != nil {
			return
		}
	}()

	// Immediately send the current track to the newly connected client
	currentTrack, err := spotifyClient.PlayerCurrentlyPlaying()
	if err != nil {
		log.Printf("Error getting currently playing track: %v", err)
		return
	}

	// Send the current track information to the client
	err = ws.WriteJSON(currentTrack)
	if err != nil {
		log.Printf("Error sending current track to client: %v", err)
		return
	}

	// Keep the connection open to listen for incoming messages (heartbeat)
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			disconnect <- ws
			break
		}
	}
}

// ConnectionManager manages client connections and disconnections using channels
func ConnectionManager() {
	for {
		select {
		case client := <-connect:
			clientsMutex.Lock()
			clients[client] = true
			clientsMutex.Unlock()
		case client := <-disconnect:
			clientsMutex.Lock()
			if _, ok := clients[client]; ok {
				delete(clients, client)
				err := client.Close()
				if err != nil {
					return
				}
			}
			clientsMutex.Unlock()
		}
	}
}

// MessageHandler continuously listens for messages on the broadcast channel and sends them to all connected clients
func MessageHandler() {
	for msg := range broadcast {
		clientsMutex.Lock()
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				err := client.Close()
				if err != nil {
					return
				}
				delete(clients, client)
			}
		}
		clientsMutex.Unlock()
	}
}

// TrackFetcher periodically fetches the currently playing track from the Spotify API and broadcasts it to clients
func TrackFetcher() {
	var playing bool
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

		// Check if the track is playing
		switch {
		case current.Playing:
			broadcast <- current
			playing = true
			// Send updates every 3 seconds while playing
			for current.Playing {
				time.Sleep(3 * time.Second)
				current, err = spotifyClient.PlayerCurrentlyPlaying()
				if err != nil {
					log.Printf("Error getting currently playing track: %v", err)
					break
				}
				broadcast <- current
			}
		case !current.Playing && playing:
			playing = false
		}

		// Wait before checking again to avoid overwhelming the API
		time.Sleep(3 * time.Second)
	}
}
