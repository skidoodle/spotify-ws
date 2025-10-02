package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"spotify-ws/internal/config"
	"spotify-ws/internal/spotify"
	"spotify-ws/internal/websocket"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	handlerOptions := &slog.HandlerOptions{Level: cfg.LogLevel}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, handlerOptions))
	slog.SetDefault(logger)

	spotifyClient := spotify.NewClient(ctx, cfg.Spotify.ClientID, cfg.Spotify.ClientSecret, cfg.Spotify.RefreshToken)
	wsServer := websocket.NewServer(":"+cfg.ServerPort, cfg.AllowedOrigins, spotifyClient, cfg.RT)

	slog.Info("starting spotify-ws server", "port", cfg.ServerPort, "realtime", cfg.RT)

	if err := wsServer.Run(ctx); err != nil {
		return fmt.Errorf("server runtime error: %w", err)
	}

	slog.Info("server shut down gracefully")
	return nil
}
