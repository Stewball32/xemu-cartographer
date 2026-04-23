package websocket

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/coder/websocket"
	"github.com/pocketbase/pocketbase/core"
)

const (
	sendBufSize  = 256
	readLimit    = 4096
	writeTimeout = 10 * time.Second
)

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	user *core.Record // nil for anonymous connections
}

// UserID returns the authenticated user's record ID, or "" for anonymous.
func (c *Client) UserID() string {
	if c.user != nil {
		return c.user.Id
	}
	return ""
}

// readPump reads messages from the browser and forwards them to the Hub.
// Runs on the handler goroutine until the connection closes.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.conn.SetReadLimit(readLimit)

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == -1 {
				log.Printf("ws: read error: %v", err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("ws: invalid message: %v", err)
			continue
		}

		c.hub.incoming <- incomingMsg{msg: msg, sender: c}
	}
}

// writePump sends queued messages from the Hub to the browser.
// Runs as a goroutine until the send channel is closed or context is cancelled.
func (c *Client) writePump(ctx context.Context) {
	defer c.conn.Close(websocket.StatusNormalClosure, "")

	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				log.Printf("ws: write error: %v", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
