package websocket

import (
	"log/slog"
	"net/http"

	"golang.org/x/net/websocket"
)

// newWebsocketHandler creates a new WebSocket handler closure.
func (s *Server) newWebsocketHandler() websocket.Handler {
	return func(ws *websocket.Conn) {
		defer func() {
			s.hub.unregister <- ws
			if err := ws.Close(); err != nil {
				slog.Warn("error while closing websocket connection", "error", err, "remoteAddr", ws.RemoteAddr())
			}
		}()

		origin := ws.Config().Origin.String()
		if !s.originChecker(origin) {
			slog.Warn("origin not allowed, rejecting connection", "origin", origin)
			return
		}

		s.hub.register <- ws

		// Send the last known state immediately upon connection.
		s.poller.SendLastState(ws)

		// Block by reading from the client to detect disconnection.
		var msg string
		for {
			if err := websocket.Message.Receive(ws, &msg); err != nil {
				break // Client has disconnected.
			}
		}
	}
}

// healthHandler responds to Docker health checks.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Warn("failed to write health check response", "error", err)
	}
}
