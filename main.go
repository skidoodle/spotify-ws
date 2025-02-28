package main

import (
	"context"
	"fmt"
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

// Configuration holds application settings
type Configuration struct {
	ServerPort     string
	AllowedOrigins []string
	LogLevel       logrus.Level
	Spotify        struct {
		ClientID     string
		ClientSecret string
		RefreshToken string
	}
}

var (
	config       Configuration
	clients      = make(map[*websocket.Conn]bool)
	clientsMutex sync.RWMutex
	broadcast    = make(chan *spotify.CurrentlyPlaying)
	connectChan  = make(chan *websocket.Conn)

	upgrader = websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
	}

	spotifyClient spotify.Client
	tokenSource   oauth2.TokenSource
	lastState     struct {
		sync.RWMutex
		track   *spotify.CurrentlyPlaying
		playing bool
	}
)

const (
	defaultPort       = ":3000"
	tokenRefreshURL   = "https://accounts.spotify.com/api/token"
	apiRetryDelay     = 3 * time.Second
	heartbeatInterval = 3 * time.Second
	writeTimeout      = 10 * time.Second
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initializeApplication()
	initializeSpotifyClient()

	router := http.NewServeMux()
	router.HandleFunc("/", connectionHandler)
	router.HandleFunc("/health", healthHandler)

	server := &http.Server{
		Addr:    config.ServerPort,
		Handler: router,
	}

	go trackFetcher(ctx)
	go messageHandler(ctx)
	go connectionManager(ctx)

	startServer(server, ctx)
	handleShutdown(server, cancel)
}

func initializeApplication() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
		ForceColors:     true,
	})

	if err := loadConfiguration(); err != nil {
		logrus.Fatal(err)
	}

	upgrader.CheckOrigin = func(r *http.Request) bool {
		if len(config.AllowedOrigins) == 0 {
			return true
		}
		origin := r.Header.Get("Origin")
		for _, allowed := range config.AllowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	}
}

func loadConfiguration() error {
	_ = godotenv.Load()

	required := map[string]*string{
		"CLIENT_ID":     &config.Spotify.ClientID,
		"CLIENT_SECRET": &config.Spotify.ClientSecret,
		"REFRESH_TOKEN": &config.Spotify.RefreshToken,
	}

	for key, ptr := range required {
		value := os.Getenv(key)
		if value == "" {
			return fmt.Errorf("missing required environment variable: %s", key)
		}
		*ptr = value
	}

	config.ServerPort = defaultPort
	if port := os.Getenv("SERVER_PORT"); port != "" {
		config.ServerPort = ":" + strings.TrimLeft(port, ":")
	}

	config.AllowedOrigins = strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")

	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch logLevel {
	case "debug":
		config.LogLevel = logrus.DebugLevel
	case "warn":
		config.LogLevel = logrus.WarnLevel
	case "error":
		config.LogLevel = logrus.ErrorLevel
	default:
		config.LogLevel = logrus.InfoLevel
	}
	logrus.SetLevel(config.LogLevel)

	return nil
}

func initializeSpotifyClient() {
	token := &oauth2.Token{RefreshToken: config.Spotify.RefreshToken}
	oauthConfig := &oauth2.Config{
		ClientID:     config.Spotify.ClientID,
		ClientSecret: config.Spotify.ClientSecret,
		Endpoint:     oauth2.Endpoint{TokenURL: tokenRefreshURL},
	}

	tokenSource = oauthConfig.TokenSource(context.Background(), token)
	spotifyClient = spotify.NewClient(oauth2.NewClient(context.Background(), tokenSource))

	logrus.Info("Spotify client initialized successfully")
}

func startServer(server *http.Server, _ context.Context) {
	go func() {
		logrus.Infof("Server starting on %s", config.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Server failed to start: %v", err)
		}
	}()
}

func handleShutdown(server *http.Server, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logrus.Info("Initiating graceful shutdown...")
	cancel()

	ctx, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()

	clientsMutex.Lock()
	for client := range clients {
		client.Close()
	}
	clientsMutex.Unlock()

	if err := server.Shutdown(ctx); err != nil {
		logrus.Errorf("Server shutdown error: %v", err)
	}
	logrus.Info("Server shutdown complete")
}

func connectionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Source", "github.com/skidoodle/spotify-ws")
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.Errorf("WebSocket upgrade failed: %v", err)
		return
	}

	// Add client to the pool
	clientsMutex.Lock()
	clients[ws] = true
	clientsMutex.Unlock()

	logrus.Debugf("New client connected: %s", ws.RemoteAddr())

	// Send initial state if available
	sendInitialState(ws)

	// Start monitoring the connection
	go monitorConnection(ws)
}

func connectionManager(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-connectChan:
			// Add client to the pool
			clientsMutex.Lock()
			clients[client] = true
			clientsMutex.Unlock()

			logrus.Debugf("New client connected: %s", client.RemoteAddr())

			// Send initial state if available
			sendInitialState(client)

			// Start monitoring the connection
			go monitorConnection(client)
		}
	}
}

func monitorConnection(ws *websocket.Conn) {
	defer func() {
		// Clean up the connection
		clientsMutex.Lock()
		delete(clients, ws)
		clientsMutex.Unlock()

		// Close the WebSocket connection
		ws.Close()
		logrus.Debugf("Client disconnected: %s", ws.RemoteAddr())
	}()

	for {
		// Set a read deadline to detect dead connections
		ws.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Attempt to read a message (even though we don't expect any)
		_, _, err := ws.NextReader()
		if err != nil {
			// Check if the error is a normal closure
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logrus.Debugf("Client disconnected unexpectedly: %v", err)
			}
			break
		}
	}
}

func sendInitialState(client *websocket.Conn) {
	lastState.RLock()
	defer lastState.RUnlock()

	if lastState.track == nil {
		logrus.Debug("No initial state available to send")
		return
	}

	if err := client.WriteJSON(lastState.track); err != nil {
		logrus.Errorf("Failed to send initial state: %v", err)
		client.Close()
	}
}

func messageHandler(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-broadcast:
			broadcastToClients(msg)
		}
	}
}

func broadcastToClients(msg *spotify.CurrentlyPlaying) {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()

	for client := range clients {
		client.SetWriteDeadline(time.Now().Add(writeTimeout))
		if err := client.WriteJSON(msg); err != nil {
			logrus.Debugf("Broadcast failed: %v", err)
			client.Close()
		}
	}
}

func trackFetcher(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fetchAndBroadcastState()
		}
	}
}

func fetchAndBroadcastState() {
	current, err := spotifyClient.PlayerCurrentlyPlaying()
	if err != nil {
		logrus.Errorf("Failed to fetch playback state: %v", err)
		time.Sleep(apiRetryDelay)
		return
	}

	logrus.Debugf("Fetched playback state: %+v", current)

	updateState(current)
}

func updateState(current *spotify.CurrentlyPlaying) {
	lastState.Lock()
	defer lastState.Unlock()

	if current == nil {
		logrus.Warn("Received nil playback state from Spotify")
		return
	}

	if lastState.track == nil {
		lastState.track = &spotify.CurrentlyPlaying{}
	}

	stateChanged := lastState.track.Item == nil ||
		current.Item == nil ||
		lastState.track.Item.ID != current.Item.ID ||
		lastState.playing != current.Playing

	lastState.track = current
	lastState.playing = current.Playing

	if stateChanged || current.Playing {
		broadcast <- current
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
