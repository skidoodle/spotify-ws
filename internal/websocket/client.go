package websocket

import (
	"log/slog"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

const (
	writeWait = 10 * time.Second
	pongWait  = 60 * time.Second
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

// close is a thread-safe method to clean up the client's resources.
// It ensures that the unregister and connection close operations happen exactly once.
func (c *Client) close() {
	c.closeOnce.Do(func() {
		slog.Debug("closing client connection", "remoteAddr", c.conn.RemoteAddr())
		c.hub.unregister <- c
		if err := c.conn.Close(); err != nil {
			// This error is expected if the other end has already hung up.
			slog.Debug("error while closing client connection", "error", err, "remoteAddr", c.conn.RemoteAddr())
		}
	})
}

// readPump is responsible for detecting a dead connection via read deadlines.
func (c *Client) readPump() {
	defer c.close()

	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		slog.Warn("failed to set initial read deadline", "error", err, "remoteAddr", c.conn.RemoteAddr())
		return
	}

	var msg string
	for {
		if err := websocket.Message.Receive(c.conn, &msg); err != nil {
			slog.Debug("client read error, triggering disconnect", "error", err, "remoteAddr", c.conn.RemoteAddr())
			break
		}
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			slog.Warn("failed to reset read deadline", "error", err, "remoteAddr", c.conn.RemoteAddr())
			break
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	defer c.close()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				slog.Debug("hub closed channel, closing connection", "remoteAddr", c.conn.RemoteAddr())
				return
			}

			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				slog.Warn("failed to set write deadline", "error", err, "remoteAddr", c.conn.RemoteAddr())
				return
			}

			if err := websocket.Message.Send(c.conn, string(message)); err != nil {
				slog.Warn("client write error", "error", err, "remoteAddr", c.conn.RemoteAddr())
				return
			}
		}
	}
}
