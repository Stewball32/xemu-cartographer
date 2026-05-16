package websocket

import (
	"log"
	"os"
	"strings"

	"github.com/coder/websocket"
	"github.com/pocketbase/pocketbase/core"
)

// ConnectHook is invoked once per accepted WebSocket connection, after the
// client has been registered with the Hub but before the read loop starts.
// `send` is a per-client non-blocking enqueue: dropped silently (with a log)
// if the client's send buffer is already full.
//
// Used for protocol bootstrapping — the scraper subsystem registers a hook
// that emits its `hello` envelope so a fresh client knows the server's
// protocol version and per-instance started_at before any other traffic.
type ConnectHook func(send func(data []byte))

// NewHandler returns a PocketBase route handler that upgrades HTTP connections
// to WebSocket. It validates an optional ?token= query param for JWT auth.
//
// Origin policy: if WS_ALLOWED_ORIGINS is set (comma-separated), those patterns
// are used. Otherwise all origins are accepted for development convenience.
//
// hooks fire in order on each new connection between client registration and
// the read loop start. Each hook receives a per-client send function it can
// use to push pre-marshaled bytes (typically a `hello` envelope).
func NewHandler(hub *Hub, app core.App, hooks ...ConnectHook) func(*core.RequestEvent) error {
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

		if len(hooks) > 0 {
			send := func(data []byte) {
				select {
				case client.send <- data:
				default:
					log.Printf("ws: connect-hook send dropped (buffer full)")
				}
			}
			for _, hook := range hooks {
				hook(send)
			}
		}

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
