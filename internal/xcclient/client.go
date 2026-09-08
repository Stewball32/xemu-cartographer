// Package xcclient is the league's consumer of an xc-scraper daemon
// (DESIGN-STEP8 §8): one WebSocket stream connection whose frames refill a
// local Mirror (§8.2) and are handed to an OnFrame hook (rebroadcast, F3b),
// plus the small HTTP reads the stream cannot answer (started_at on
// summary-add). It is PocketBase-free by construction (deps_test.go): the
// flagship hub is reached through the nil-safe HubPort only.
package xcclient

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

const (
	// DefaultStaleAfter is D-12: disconnected longer than this clears the mirror.
	DefaultStaleAfter = 60 * time.Second
	// DefaultFrameBuffer is the read-pump → worker channel depth (§8.1).
	DefaultFrameBuffer = 4096
	// HostrunnerRoom is the daemon's extension room for host-runner events (D-9).
	HostrunnerRoom = "xc:hostrunner"
	// EvictReason is the close reason downstream clients see on an upstream
	// (re)connect or per-instance epoch change; the status is 1012.
	EvictReason = "upstream resync"

	pingInterval = 15 * time.Second
	backoffMin   = time.Second
	backoffMax   = 30 * time.Second
	dialTimeout  = 10 * time.Second
	writeTimeout = 5 * time.Second
	fetchTimeout = 3 * time.Second
	fetchRetry   = 5 * time.Second
	readLimit    = 64 << 20
	// stableAfter is how long a connection must live before the backoff resets.
	stableAfter = 10 * time.Second
)

// AlwaysClasses are joined for every instance on connect and never left
// (hybrid always-classes, §8.1).
var AlwaysClasses = []string{
	wire.ClassGame,
	wire.ClassEvent,
	wire.ClassPreviousGame,
	wire.ClassXbox,
	wire.ClassScenario,
}

// HubPort is what the client needs from the flagship hub (*ws.Hub satisfies
// it). A nil Hub is allowed: sends and evictions become no-ops.
type HubPort interface {
	SendToRoomRaw(room string, data []byte)
	EvictRoomPrefix(prefix string, status websocket.StatusCode, reason string) int
}

// Frame is one upstream frame as handed to Config.OnFrame after the mirror
// update: Raw is the exact bytes received, Msg the decoded outer message and
// Env the decoded envelope header (nil for non-"scraper" frames such as
// type:"host_runner").
type Frame struct {
	Raw []byte
	Msg wire.Message
	Env *wire.Envelope
}

// Config configures New.
type Config struct {
	// URL is XC_SCRAPER_URL: http(s)://host[:port][/prefix]. The stream is
	// dialled at <URL>/api/ws (ws/wss) and HTTP reads at <URL>/api/....
	URL string
	// Token is the daemon feed token (--token); "" when the daemon is open.
	Token string
	// StaleAfter is D-12; zero means DefaultStaleAfter.
	StaleAfter time.Duration
	// Hub receives evictions (and, via F3b, rebroadcasts). nil-safe.
	Hub HubPort
	// OnFrame runs on the worker goroutine for every frame after the mirror
	// update (hello included; Env nil for non-scraper types such as
	// host_runner). Upstream error frames are logged, not handed on.
	OnFrame func(Frame)
	// Log receives one-line diagnostics; nil means log.Printf.
	Log func(format string, args ...any)
	// Hostrunner joins xc:hostrunner on connect (daemon --hostrunner, D-9).
	Hostrunner bool
	// HTTPClient serves the WebSocket dial and the /api/instances reads; nil
	// means http.DefaultClient.
	HTTPClient *http.Client
}

// Client is one upstream stream connection plus its Mirror. Create with New,
// drive with Run.
type Client struct {
	cfg    Config
	base   *url.URL
	wsURL  string
	http   *http.Client
	mirror *Mirror
	logf   func(string, ...any)

	frames   chan streamItem
	gen      atomic.Uint64
	running  atomic.Bool
	shed     atomic.Uint64
	seqGaps  atomic.Uint64
	frameBuf int

	mu          sync.Mutex
	conn        *websocket.Conn
	wanted      map[string]bool
	hooks       []func()
	connected   bool
	since       time.Time
	reconnects  int
	lastFrameAt time.Time
	stale       bool
	gapLogAt    time.Time
	fetchAt     time.Time

	// tunables (tests)
	staleAfter   time.Duration
	pingEvery    time.Duration
	backoffMin   time.Duration
	backoffMax   time.Duration
	fetchTimeout time.Duration
	fetchRetry   time.Duration
}

// New validates cfg and returns an idle client. URL must be http(s)://
// (or ws(s)://) with a host; the stream endpoint is derived from it.
func New(cfg Config) (*Client, error) {
	base, wsURL, err := Endpoints(cfg.URL, cfg.Token)
	if err != nil {
		return nil, err
	}
	c := &Client{
		cfg:          cfg,
		base:         base,
		wsURL:        wsURL,
		http:         cfg.HTTPClient,
		mirror:       NewMirror(),
		logf:         cfg.Log,
		frameBuf:     DefaultFrameBuffer,
		wanted:       make(map[string]bool),
		staleAfter:   cfg.StaleAfter,
		pingEvery:    pingInterval,
		backoffMin:   backoffMin,
		backoffMax:   backoffMax,
		fetchTimeout: fetchTimeout,
		fetchRetry:   fetchRetry,
	}
	if c.http == nil {
		c.http = http.DefaultClient
	}
	if c.logf == nil {
		c.logf = log.Printf
	}
	if c.staleAfter <= 0 {
		c.staleAfter = DefaultStaleAfter
	}
	c.since = time.Now()
	return c, nil
}

// Endpoints derives the HTTP base and the stream URL from XC_SCRAPER_URL
// (§11): http→ws, https→wss, path prefix kept, token as ?token=.
func Endpoints(raw, token string) (base *url.URL, wsURL string, err error) {
	if strings.TrimSpace(raw) == "" {
		return nil, "", errors.New("xcclient: empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, "", fmt.Errorf("xcclient: parse URL: %w", err)
	}
	if u.Host == "" {
		return nil, "", fmt.Errorf("xcclient: URL %q has no host", raw)
	}
	ws := *u
	switch u.Scheme {
	case "http":
		ws.Scheme = "ws"
	case "https":
		ws.Scheme = "wss"
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	default:
		return nil, "", fmt.Errorf("xcclient: URL %q: scheme must be http(s)", raw)
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawQuery, u.Fragment, u.RawPath = "", "", ""
	ws.Path = path.Join(u.Path, "/api/ws")
	ws.RawPath, ws.Fragment = "", ""
	q := url.Values{}
	if token != "" {
		q.Set("token", token)
	}
	ws.RawQuery = q.Encode()
	return u, ws.String(), nil
}

// Mirror returns the client's mirror (shared, read-safe).
func (c *Client) Mirror() *Mirror { return c.mirror }

// OnConnect registers fn to run on the worker goroutine after every
// successful hello + joins (config push §10, F5).
func (c *Client) OnConnect(fn func()) {
	if fn == nil {
		return
	}
	c.mu.Lock()
	c.hooks = append(c.hooks, fn)
	c.mu.Unlock()
}

// Join adds room to the demand set (re-joined on every reconnect) and sends
// join_room upstream when connected (demand layer, F3b).
func (c *Client) Join(room string) {
	c.mu.Lock()
	c.wanted[room] = true
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		c.send(conn, wire.Message{Type: wire.TypeJoinRoom, Room: room})
	}
}

// Leave removes room from the demand set and sends leave_room upstream
// unless the room is an always-room (host:summary, always-classes,
// xc:hostrunner), which are never left.
func (c *Client) Leave(room string) {
	c.mu.Lock()
	delete(c.wanted, room)
	conn := c.conn
	c.mu.Unlock()
	if conn == nil || c.isAlwaysRoom(room) {
		return
	}
	c.send(conn, wire.Message{Type: wire.TypeLeaveRoom, Room: room})
}

func (c *Client) isAlwaysRoom(room string) bool {
	if room == wire.SummaryRoom || (room == HostrunnerRoom && c.cfg.Hostrunner) {
		return true
	}
	rm, err := wire.ParseRoom(room)
	if err != nil || !rm.IsHost() || rm.Class == "" {
		return false
	}
	for _, class := range AlwaysClasses {
		if rm.Class == class {
			return true
		}
	}
	return false
}

// alwaysRooms are the per-instance rooms joined on connect / summary-add.
func alwaysRooms(instance string) []string {
	out := make([]string, 0, len(AlwaysClasses))
	for _, class := range AlwaysClasses {
		room, err := wire.RoomForInstanceClass(instance, class)
		if err != nil {
			continue
		}
		out = append(out, room)
	}
	return out
}

// evict closes every downstream client holding a room with prefix (1012,
// no error frame; F2 EvictRoomPrefix). No-op without a hub.
func (c *Client) evict(prefix, why string) {
	if c.cfg.Hub == nil {
		return
	}
	if n := c.cfg.Hub.EvictRoomPrefix(prefix, websocket.StatusServiceRestart, EvictReason); n > 0 {
		c.logf("xcclient: evicted %d downstream client(s) on %s* (%s)", n, prefix, why)
	}
}

func (c *Client) emit(f Frame) {
	if c.cfg.OnFrame != nil {
		c.cfg.OnFrame(f)
	}
}
