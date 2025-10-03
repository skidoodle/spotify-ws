package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultPort    = "3000"
	requestTimeout = 5 * time.Second
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	if err := run(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "health check failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(ctx context.Context) error {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = defaultPort
	}

	url := fmt.Sprintf("http://localhost:%s/health", port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", url, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: failed to close response body: %v\n", closeErr)
		}
	}()

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("failed to discard response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	return nil
}
