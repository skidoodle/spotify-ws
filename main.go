package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

const (
	serverPort      = ":3000"
	tokenRefreshURL = "https://accounts.spotify.com/api/token"
	apiRetryDelay   = 3 * time.Second
	heartbeatDelay  = 3 * time.Second
)

var (
	clients      = make(map[*websocket.Conn]bool)
	clientsMutex sync.RWMutex
	broadcast    = make(chan *spotify.CurrentlyPlaying)
	connect      = make(chan *websocket.Conn)
	disconnect   = make(chan *websocket.Conn)

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
	}

	spotifyClient    spotify.Client
	tokenSource      oauth2.TokenSource
	config           *oauth2.Config
	lastPlayingState *bool
	lastTrackState   *spotify.CurrentlyPlaying
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	loadEnv()

	config = &oauth2.Config{
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenRefreshURL,
		},
	}
	token := &oauth2.Token{RefreshToken: os.Getenv("REFRESH_TOKEN")}
	tokenSource = config.TokenSource(context.Background(), token)

	httpClient := oauth2.NewClient(context.Background(), tokenSource)
	spotifyClient = spotify.NewClient(httpClient)

	http.HandleFunc("/", connectionHandler)
	http.HandleFunc("/health", healthHandler)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go trackFetcher()
	go messageHandler()
	go connectionManager()

	server := &http.Server{
		Addr: serverPort,
	}

	go func() {
		logrus.Infof("Server started on %s", serverPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Server failed: %v", err)
		}
	}()

	<-stop
	logrus.Info("Shutting down server...")

	clientsMutex.Lock()
	for client := range clients {
		_ = client.Close()
	}
	clientsMutex.Unlock()

	if err := server.Shutdown(context.Background()); err != nil {
		logrus.Fatalf("Server shutdown failed: %v", err)
	}
	logrus.Info("Server exited cleanly")
}

func loadEnv() {
	if err := godotenv.Load(); err != nil {
		logrus.Warn("Could not load .env file, falling back to system environment")
	}

	requiredVars := []string{"CLIENT_ID", "CLIENT_SECRET", "REFRESH_TOKEN"}
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			logrus.Fatalf("Missing required environment variable: %s", v)
		}
	}

	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	if logLevel == "debug" {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Info("Log level set to DEBUG")
	} else {
		logrus.SetLevel(logrus.InfoLevel)
		logrus.Info("Log level set to INFO")
	}
}

func connectionHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.Errorf("Failed to upgrade to WebSocket: %v", err)
		return
	}
	connect <- ws

	clientsMutex.RLock()
	if lastTrackState != nil {
		if err := ws.WriteJSON(lastTrackState); err != nil {
			logrus.Errorf("Failed to send initial state to client: %v", err)
		}
	}
	clientsMutex.RUnlock()

	defer func() {
		disconnect <- ws
	}()

	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}
}

func connectionManager() {
	for {
		select {
		case client := <-connect:
			clientsMutex.Lock()
			clients[client] = true
			clientsMutex.Unlock()
			logrus.Debugf("New client connected: %v", client.RemoteAddr())
		case client := <-disconnect:
			clientsMutex.Lock()
			if _, ok := clients[client]; ok {
				delete(clients, client)
				logrus.Debugf("Client disconnected: %v", client.RemoteAddr())
			}
			clientsMutex.Unlock()
		}
	}
}

func messageHandler() {
	for msg := range broadcast {
		clientsMutex.RLock()
		for client := range clients {
			if err := client.WriteJSON(msg); err != nil {
				logrus.Errorf("Failed to send message to client: %v", err)
				disconnect <- client
			}
		}
		clientsMutex.RUnlock()
	}
}

func trackFetcher() {
	for {
		current, err := fetchCurrentlyPlaying()
		if err != nil {
			logrus.Errorf("Error fetching currently playing track: %v", err)
			time.Sleep(apiRetryDelay)
			continue
		}

		if current != nil {
			clientsMutex.Lock()
			lastTrackState = current
			clientsMutex.Unlock()

			if lastPlayingState == nil || *lastPlayingState != current.Playing {
				logrus.Debugf("Playback state changed: is_playing=%v", current.Playing)
				broadcast <- current
				lastPlayingState = &current.Playing
			}

			if current.Playing {
				broadcast <- current
				time.Sleep(heartbeatDelay)
				continue
			}
		}
		time.Sleep(heartbeatDelay)
	}
}

func fetchCurrentlyPlaying() (*spotify.CurrentlyPlaying, error) {
	current, err := spotifyClient.PlayerCurrentlyPlaying()
	if err != nil && err.Error() == "token expired" {
		logrus.Warn("Spotify token expired, refreshing token...")
		newToken, err := tokenSource.Token()
		if err != nil {
			return nil, err
		}
		httpClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(newToken))
		spotifyClient = spotify.NewClient(httpClient)
		return spotifyClient.PlayerCurrentlyPlaying()
	}
	return current, err
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
