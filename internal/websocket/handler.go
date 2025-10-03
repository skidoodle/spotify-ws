package websocket

import (
	"log/slog"
	"net/http"

	"golang.org/x/net/websocket"
)

// newWebsocketHandler creates the handler that upgrades connections.
func (s *Server) newWebsocketHandler() websocket.Handler {
	return func(ws *websocket.Conn) {
		origin := ws.Config().Origin.String()
		if !s.originChecker(origin) {
			slog.Warn("origin not allowed, rejecting connection", "origin", origin)
			return
		}

		slog.Debug("client connected, upgrading connection", "remoteAddr", ws.RemoteAddr())

		client := &Client{hub: s.hub, conn: ws, send: make(chan []byte, 256)}

		client.hub.register <- client

		go client.writePump()
		client.readPump()
	}
}

// healthHandler responds to Docker health checks.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Warn("failed to write health check response", "error", err)
	}
}
