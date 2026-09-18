package websocket

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
)

// ConnectHook is invoked once per accepted WebSocket connection, after the
// client has been registered with the Hub but before the read loop starts.
// `send` is a per-client non-blocking enqueue: dropped silently (with a log)
// if the client's send buffer is already full. `p` is the principal the
// connection resolved to, so a hook can narrow what it announces.
//
// Used for protocol bootstrapping — the scraper subsystem registers a hook
// that emits its `hello` envelope so a fresh client knows the server's
// protocol version and per-instance started_at before any other traffic
// (filtered to the instances p may join).
type ConnectHook func(send func(data []byte), p authz.Principal)

// reResolveInterval is how often each connection re-derives its principal
// (authz.ReResolveInterval). A variable so handler_test can tick fast.
var reResolveInterval = authz.ReResolveInterval

// NewHandler returns a PocketBase route handler that upgrades HTTP connections
// to WebSocket. The connection's principal is resolved by pb.ResolveWS from
// the query string — ?token= (PB JWT or opaque key), ?spectator=, or the
// ?console=<name> door — against the process-wide authz adapter read from
// pb.Default() at connect time; a resolve failure (bad or banned token) is
// logged and the socket proceeds as Nobody, which can join public rooms and
// nothing else.
//
// Origin policy: if WS_ALLOWED_ORIGINS is set (comma-separated), those patterns
// are used. Otherwise all origins are accepted for development convenience.
//
// hooks fire in order on each new connection between client registration and
// the read loop start. Each hook receives a per-client send function it can
// use to push pre-marshaled bytes (typically a `hello` envelope) and the
// connection's principal.
func NewHandler(hub *Hub, app core.App, hooks ...ConnectHook) func(*core.RequestEvent) error {
	opts := buildAcceptOptions()
	interval := reResolveInterval

	return func(e *core.RequestEvent) error {
		d := pb.Default()
		principal, err := pb.ResolveWS(app, d, e.Request)
		if err != nil {
			log.Printf("ws: connect: %v", err)
			principal = authz.Nobody()
		}

		conn, err := websocket.Accept(e.Response, e.Request, opts)
		if err != nil {
			return err
		}

		client := newClient(hub, conn, principal)

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
				hook(send, principal)
			}
		}

		// The request context is cancelled when this handler returns; the
		// explicit cancel also stops the re-resolve loop the moment the read
		// loop ends, without waiting on net/http. The loop is joined before
		// returning so no tick can outlive its connection.
		ctx, cancel := context.WithCancel(e.Request.Context())
		defer cancel()

		loopDone := make(chan struct{})
		go func() {
			defer close(loopDone)
			client.reResolveLoop(ctx, app, d, interval)
		}()
		go client.writePump(ctx)
		client.readPump(ctx) // Blocks until disconnect.

		cancel()
		<-loopDone
		return nil
	}
}

// reResolveLoop re-derives the connection's principal every interval
// (authz.ReResolveInterval; DESIGN-STEP6 §7.3 W-2). A credential that no
// longer resolves — revoked or expired key, banned or deleted user — evicts
// the socket with error{code:"session_revoked"} and close code 4401. A
// principal that still resolves is swapped in and every room the connection
// holds (host: and admin alike) is re-decided with room.join, leaving the
// ones it can no longer enter (role strip, roster loss past the grace
// window, a console re-bound elsewhere). Exits when ctx is cancelled (the
// read loop ended).
func (c *Client) reResolveLoop(ctx context.Context, app core.App, d *pb.PBDeps, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.reResolve(ctx, app, d) {
				return
			}
		}
	}
}

// reResolve is one tick of reResolveLoop; false means the client was
// evicted and the loop should stop.
func (c *Client) reResolve(ctx context.Context, app core.App, d *pb.PBDeps) bool {
	current := c.Principal()
	next, ok := pb.ReResolve(app, d, current)
	if !ok {
		log.Printf("ws: session revoked kind=%s id=%s", current.Kind, current.ID)
		c.evict(ctx, "session_revoked", "credential no longer valid")
		return false
	}
	c.setPrincipal(next)
	if left := c.hub.recheckRooms(c); len(left) > 0 {
		log.Printf("ws: re-resolve left rooms %v kind=%s id=%s", left, next.Kind, next.ID)
	}
	return true
}

// buildAcceptOptions reads WS_ALLOWED_ORIGINS and returns websocket.AcceptOptions.
func buildAcceptOptions() *websocket.AcceptOptions {
	origins := os.Getenv("WS_ALLOWED_ORIGINS")
	if origins == "" {
		// FAIL-OPEN default (dev convenience): with no allowlist the handshake
		// accepts ALL origins (InsecureSkipVerify), widening the cross-site
		// WebSocket-hijacking surface. Auth (JWT / overlay token) is still
		// required, so this is not an open door — but it is looser than a prod
		// deploy wants. Behavior is intentionally unchanged; this only warns.
		// Recommendation: set WS_ALLOWED_ORIGINS to your public origin(s).
		log.Printf("SECURITY WARNING: WS_ALLOWED_ORIGINS unset — the WebSocket endpoint accepts all origins (InsecureSkipVerify). Set it to your public origin(s) in production.")
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
