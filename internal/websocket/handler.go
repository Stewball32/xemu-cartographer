package websocket

import (
	"log"
	"os"
	"strings"

	"github.com/coder/websocket"
	"github.com/pocketbase/pocketbase/core"
)

// NewHandler returns a PocketBase route handler that upgrades HTTP connections
// to WebSocket. It validates an optional ?token= query param for JWT auth.
//
// Origin policy: if WS_ALLOWED_ORIGINS is set (comma-separated), those patterns
// are used. Otherwise all origins are accepted for development convenience.
func NewHandler(hub *Hub, app core.App) func(*core.RequestEvent) error {
	opts := buildAcceptOptions()

	return func(e *core.RequestEvent) error {
		var user *core.Record
		if token := e.Request.URL.Query().Get("token"); token != "" {
			record, err := app.FindAuthRecordByToken(token, core.TokenTypeAuth)
			if err != nil {
				log.Printf("ws: invalid auth token: %v", err)
			} else {
				user = record
			}
		}

		conn, err := websocket.Accept(e.Response, e.Request, opts)
		if err != nil {
			return err
		}

		client := &Client{
			hub:  hub,
			conn: conn,
			send: make(chan []byte, sendBufSize),
			user: user,
		}

		hub.register <- client

		ctx := e.Request.Context()
		go client.writePump(ctx)
		client.readPump(ctx) // Blocks until disconnect.

		return nil
	}
}

// buildAcceptOptions reads WS_ALLOWED_ORIGINS and returns websocket.AcceptOptions.
func buildAcceptOptions() *websocket.AcceptOptions {
	origins := os.Getenv("WS_ALLOWED_ORIGINS")
	if origins == "" {
		return &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		}
	}

	var patterns []string
	for _, o := range strings.Split(origins, ",") {
		if p := strings.TrimSpace(o); p != "" {
			patterns = append(patterns, p)
		}
	}

	return &websocket.AcceptOptions{
		OriginPatterns: patterns,
	}
}
