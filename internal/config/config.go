package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the application configuration.
type Config struct {
	ServerPort     string
	AllowedOrigins []string
	LogLevel       slog.Level
	RT             bool
	Spotify        struct {
		ClientID     string
		ClientSecret string
		RefreshToken string
	}
}

// Load loads the configuration from environment variables.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found, using environment variables")
	}

	cfg := &Config{}

	cfg.Spotify.ClientID = os.Getenv("SPOTIFY_CLIENT_ID")
	cfg.Spotify.ClientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")
	cfg.Spotify.RefreshToken = os.Getenv("SPOTIFY_REFRESH_TOKEN")

	if cfg.Spotify.ClientID == "" || cfg.Spotify.ClientSecret == "" || cfg.Spotify.RefreshToken == "" {
		return nil, fmt.Errorf("spotify credentials are not set")
	}

	cfg.ServerPort = os.Getenv("SERVER_PORT")
	if cfg.ServerPort == "" {
		cfg.ServerPort = "3000"
	}

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins != "" {
		cfg.AllowedOrigins = strings.Split(allowedOrigins, ",")
	}

	rt, err := strconv.ParseBool(os.Getenv("RT"))
	if err != nil {
		cfg.RT = false
	} else {
		cfg.RT = rt
	}

	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "warn":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		cfg.LogLevel = slog.LevelInfo
	}

	return cfg, nil
}
