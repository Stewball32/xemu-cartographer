package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
)

const (
	sendBufSize  = 256
	readLimit    = 4096
	writeTimeout = 10 * time.Second

	// closeSessionRevoked is the close code sent when the periodic
	// re-resolve finds the connection's credential no longer valid (revoked
	// key, banned / deleted user). 4401 is the application-range analogue of
	// HTTP 401 (DESIGN-STEP6 §7.3 W-2).
	closeSessionRevoked websocket.StatusCode = 4401
)

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	// send is the outbound queue writePump drains. It is NEVER closed: the
	// Hub's senders enqueue from many goroutines (Run, the scraper runners
	// through the *Raw API, the re-resolve loop) and a close racing any of
	// them is a fatal "send on closed channel". Removal is signalled through
	// done instead, and a frame enqueued in the window between the closed
	// check and the removal just sits in a dead buffer.
	send chan []byte
	// done is closed exactly once (closeOnce) when the Hub removes the
	// client — disconnect, full buffer, Stop. writePump exits on it and
	// trySend refuses to enqueue after it.
	done      chan struct{}
	closeOnce sync.Once

	// principal is who this connection is, resolved once at connect
	// (pb.ResolveWS) and refreshed every authz.ReResolveInterval by the
	// re-resolve loop, which swaps it under mu. Every authz decision the Hub
	// and the handlers make reads it through Principal(); nothing else on
	// the connection carries identity.
	mu        sync.RWMutex
	principal authz.Principal
}

// newClient builds a Client with its queues initialised; hub registration
// is the caller's.
func newClient(hub *Hub, conn *websocket.Conn, p authz.Principal) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, sendBufSize),
		done:      make(chan struct{}),
		principal: p,
	}
}

// markClosed signals writePump to stop and trySend to drop; idempotent.
func (c *Client) markClosed() {
	c.closeOnce.Do(func() { close(c.done) })
}

// closed reports whether the Hub has removed the client.
func (c *Client) closed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Principal returns the connection's current principal (a value copy).
func (c *Client) Principal() authz.Principal {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.principal
}

// setPrincipal replaces the connection's principal (re-resolve tick).
func (c *Client) setPrincipal(p authz.Principal) {
	c.mu.Lock()
	c.principal = p
	c.mu.Unlock()
}

// UserID returns the principal's users.id, or "" when the connection is not
// backed by a users record (anonymous, token kinds without a user, …).
func (c *Client) UserID() string {
	return c.Principal().UserID
}

// readPump reads messages from the browser and forwards them to the Hub.
// Runs on the handler goroutine until the connection closes.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.requestUnregister(c)
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
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

		// Once the Hub has stopped nobody drains incoming; end the read loop
		// rather than block on a queue that will never move again.
		select {
		case c.hub.incoming <- incomingMsg{msg: msg, sender: c}:
		case <-c.hub.done:
			return
		}
	}
}

// writePump sends queued messages from the Hub to the browser. Runs as a
// goroutine until the Hub removes the client (done) or the context is
// cancelled.
func (c *Client) writePump(ctx context.Context) {
	defer func() { _ = c.conn.Close(websocket.StatusNormalClosure, "") }()

	for {
		select {
		case data := <-c.send:
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				log.Printf("ws: write error: %v", err)
				return
			}
		case <-c.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// evict tells the browser why it is being cut off and closes the socket
// with closeSessionRevoked (error frame {code, message} first, then the
// close frame carrying code as its reason).
func (c *Client) evict(ctx context.Context, code, message string) {
	c.evictWith(ctx, closeSessionRevoked, code, message)
}

// evictWith closes the socket with status. When errCode is set an error
// frame {errCode, message} goes first, written directly (not queued on
// send) so it is on the wire before the close frame — coder/websocket
// serialises concurrent writers — and the close reason echoes errCode; with
// errCode empty no error frame is sent and message is the close reason
// (the 1012 "upstream resync" path, which the frontend treats as a plain
// reconnect). readPump's exit then unregisters the client from the Hub.
// A client without a socket (test fixtures) is left alone.
func (c *Client) evictWith(ctx context.Context, status websocket.StatusCode, errCode, message string) {
	if c.conn == nil {
		return
	}
	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	reason := message
	if errCode != "" {
		reason = errCode
		if data, ok := errorMessage("", errCode, message); ok {
			_ = c.conn.Write(writeCtx, websocket.MessageText, data)
		}
	}
	_ = c.conn.Close(status, reason)
}
