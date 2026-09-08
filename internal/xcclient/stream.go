package xcclient

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// streamItem is what the read pump hands the worker: a raw frame tagged with
// its connection generation, or a disconnect marker.
type streamItem struct {
	raw        []byte
	gen        uint64
	disconnect bool
}

// ErrAlreadyRunning is returned by a second concurrent Run.
var ErrAlreadyRunning = errors.New("xcclient: Run already active")

// Run dials the daemon, pumps frames into the worker and reconnects with
// 1 s→30 s backoff (+ jitter) until ctx is done. It returns ctx.Err().
func (c *Client) Run(ctx context.Context) error {
	if !c.running.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	defer c.running.Store(false)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.frames = make(chan streamItem, c.frameBuf)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.worker(ctx)
	}()

	backoff := c.backoffMin
	for {
		start := time.Now()
		err := c.connect(ctx)
		if ctx.Err() != nil {
			break
		}
		if time.Since(start) >= stableAfter {
			backoff = c.backoffMin
		}
		wait := jitter(backoff)
		c.logf("xcclient: upstream %s: %v; reconnect in %s", c.base.Host, err, wait.Round(time.Millisecond))
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
		if ctx.Err() != nil {
			break
		}
		backoff = min(backoff*2, c.backoffMax)
	}
	cancel()
	<-done
	return ctx.Err()
}

func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return d + time.Duration(rand.Int64N(int64(d)/2+1))
}

// connect runs one connection to completion: dial, read pump (with the
// ping loop beside it), then the disconnect marker. The returned error is
// why the connection ended.
func (c *Client) connect(ctx context.Context) error {
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	conn, _, err := websocket.Dial(dialCtx, c.wsURL, &websocket.DialOptions{HTTPClient: c.http})
	cancel()
	if err != nil {
		return err
	}
	conn.SetReadLimit(readLimit)
	gen := c.gen.Add(1)
	c.setConn(conn, gen)
	c.logf("xcclient: connected to %s", c.base.Host)

	connCtx, cancelConn := context.WithCancel(ctx)
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		c.pingLoop(connCtx, conn)
	}()

	var readErr error
	for {
		_, data, err := conn.Read(connCtx)
		if err != nil {
			readErr = err
			break
		}
		c.push(data, gen)
	}
	cancelConn()
	<-pingDone
	_ = conn.CloseNow()
	c.clearConn()
	select {
	case c.frames <- streamItem{disconnect: true, gen: gen}:
	case <-ctx.Done():
	}
	return readErr
}

// pingLoop detects half-open sockets: a ping that fails or times out closes
// the connection, which unblocks the read pump.
func (c *Client) pingLoop(ctx context.Context, conn *websocket.Conn) {
	t := time.NewTicker(c.pingEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, c.pingEvery)
		err := conn.Ping(pctx)
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				c.logf("xcclient: ping failed: %v", err)
				_ = conn.Close(websocket.StatusPolicyViolation, "ping timeout")
			}
			return
		}
	}
}

// push hands a frame to the worker without ever blocking: above the 75 %
// watermark firehose classes (tick/objects/debug) are shed; a full channel
// sheds anything (counted in Status.Shed).
func (c *Client) push(data []byte, gen uint64) {
	if len(c.frames)*4 > cap(c.frames)*3 && isFirehose(data) {
		c.shed.Add(1)
		return
	}
	select {
	case c.frames <- streamItem{raw: data, gen: gen}:
	default:
		c.shed.Add(1)
	}
}

// isFirehose peeks the frame's room to classify it (host:<inst>:<class>).
func isFirehose(data []byte) bool {
	var head struct {
		Room string `json:"room"`
	}
	if json.Unmarshal(data, &head) != nil {
		return false
	}
	rm, err := wire.ParseRoom(head.Room)
	if err != nil {
		return false
	}
	switch rm.Class {
	case wire.ClassTick, wire.ClassObjects, wire.ClassDebug:
		return true
	}
	return false
}

func (c *Client) setConn(conn *websocket.Conn, gen uint64) {
	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.since = time.Now()
	if gen > 1 {
		c.reconnects++
	}
	c.mu.Unlock()
}

func (c *Client) clearConn() {
	c.mu.Lock()
	c.conn = nil
	c.connected = false
	c.since = time.Now()
	c.mu.Unlock()
}

// send writes one control message (join_room / leave_room) upstream.
func (c *Client) send(conn *websocket.Conn, msg wire.Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		c.logf("xcclient: send %s %s: %v", msg.Type, msg.Room, err)
	}
}

func (c *Client) currentConn() *websocket.Conn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

// joinRooms sends join_room for each room on the current connection.
func (c *Client) joinRooms(rooms []string) {
	conn := c.currentConn()
	if conn == nil {
		return
	}
	for _, room := range rooms {
		c.send(conn, wire.Message{Type: wire.TypeJoinRoom, Room: room})
	}
}

// leaveRooms sends leave_room for each room on the current connection.
func (c *Client) leaveRooms(rooms []string) {
	conn := c.currentConn()
	if conn == nil {
		return
	}
	for _, room := range rooms {
		c.send(conn, wire.Message{Type: wire.TypeLeaveRoom, Room: room})
	}
}

func trimPayload(p []byte) string {
	s := strings.TrimSpace(string(p))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// worker is the single consumer of the frame channel: mirror update →
// OnFrame, hello / summary bookkeeping, and the stale timer (D-12).
func (c *Client) worker(ctx context.Context) {
	var staleC <-chan time.Time
	var stale *time.Timer
	defer func() {
		if stale != nil {
			stale.Stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case it := <-c.frames:
			if it.disconnect {
				if stale != nil {
					stale.Stop()
				}
				stale = time.NewTimer(c.staleAfter)
				staleC = stale.C
				continue
			}
			if it.gen != c.gen.Load() {
				continue // frame of a connection that already ended
			}
			c.handle(ctx, it.raw)
		case <-staleC:
			staleC = nil
			if c.currentConn() == nil {
				c.mirror.Clear()
				c.setStale(true)
				c.logf("xcclient: upstream stale for %s; mirror cleared", c.staleAfter)
			}
		}
	}
}

func (c *Client) handle(ctx context.Context, raw []byte) {
	var msg wire.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.logf("xcclient: undecodable frame: %v", err)
		return
	}
	c.touch()
	switch msg.Type {
	case wire.TypeScraper:
		var env wire.Envelope
		if err := json.Unmarshal(msg.Payload, &env); err != nil {
			c.logf("xcclient: undecodable envelope on %q: %v", msg.Room, err)
			return
		}
		c.handleScraper(ctx, raw, &env)
		c.emit(Frame{Raw: raw, Msg: msg, Env: &env})
	case wire.TypeError:
		c.logf("xcclient: upstream error frame: %s", trimPayload(msg.Payload))
	default:
		c.emit(Frame{Raw: raw, Msg: msg})
	}
}

func (c *Client) handleScraper(ctx context.Context, raw []byte, env *wire.Envelope) {
	switch env.Type {
	case wire.ClassHello:
		c.onHello(env)
	case wire.ClassSummary:
		c.onSummary(ctx, raw, env)
	default:
		if !wire.IsPerInstanceClass(env.Type) {
			return // events / probe replies: never cached
		}
		res := c.mirror.Store(env.Instance, env.Type, raw, env)
		if res.New {
			c.onInstanceAdded(ctx, env.Instance, "frame")
		}
		if res.Regression {
			c.onEpoch(ctx, env.Instance, "seq regression on "+env.Type)
		}
		if res.Gap > 0 {
			c.noteSeqGap(env.Instance, env.Type, res.Gap)
		}
	}
}

// onHello resets the instance set from the daemon's hello, joins the
// always rooms + the demand set, runs the connect hooks and evicts the
// downstream host:* clients so they re-hello against the refilled mirror.
func (c *Client) onHello(env *wire.Envelope) {
	var hp wire.HelloPayload
	if err := json.Unmarshal(env.Data, &hp); err != nil {
		c.logf("xcclient: undecodable hello: %v", err)
		return
	}
	if hp.ProtocolVersion != wire.ProtocolVersion {
		c.logf("xcclient: upstream protocol_version %d, this build speaks %d", hp.ProtocolVersion, wire.ProtocolVersion)
	}
	seen := make(map[string]bool, len(hp.Instances))
	for _, in := range hp.Instances {
		seen[in.Name] = true
		if c.mirror.SetInstance(in.Name, in.StartedAt, false) {
			c.logf("xcclient: %s started_at changed; cache dropped", in.Name)
		}
	}
	for _, name := range c.mirror.Instances() {
		if !seen[name] {
			c.mirror.Remove(name)
		}
	}
	rooms := []string{wire.SummaryRoom}
	for _, in := range hp.Instances {
		rooms = append(rooms, alwaysRooms(in.Name)...)
	}
	if c.cfg.Hostrunner {
		rooms = append(rooms, HostrunnerRoom)
	}
	c.mu.Lock()
	for room := range c.wanted {
		rooms = append(rooms, room)
	}
	hooks := append([]func(){}, c.hooks...)
	c.stale = false
	c.mu.Unlock()
	c.joinRooms(dedupe(rooms))
	c.logf("xcclient: hello: %d instance(s), %d room(s) joined", len(hp.Instances), len(dedupe(rooms)))
	for _, fn := range hooks {
		fn()
	}
	c.evict(wire.HostRoomPrefix+":", "upstream connect")
}

// onSummary caches the frame and diffs its hosts against the instance set:
// adds fetch started_at + join, removes drop the mirror entry + leave.
func (c *Client) onSummary(ctx context.Context, raw []byte, env *wire.Envelope) {
	var sp wire.SummaryPayload
	if err := json.Unmarshal(env.Data, &sp); err != nil {
		c.logf("xcclient: undecodable summary: %v", err)
		return
	}
	c.mirror.StoreSummary(raw, &sp)
	current := make(map[string]bool, len(sp.Hosts))
	for _, h := range sp.Hosts {
		if h.Instance != "" {
			current[h.Instance] = true
		}
	}
	known := make(map[string]bool)
	for _, name := range c.mirror.Instances() {
		known[name] = true
		if !current[name] {
			c.mirror.Remove(name)
			c.leaveRooms(alwaysRooms(name))
			c.logf("xcclient: %s left the summary; dropped", name)
		}
	}
	for _, h := range sp.Hosts {
		if h.Instance == "" || known[h.Instance] {
			continue
		}
		c.mirror.SetInstance(h.Instance, env.Ts, true)
		c.onInstanceAdded(ctx, h.Instance, "summary")
	}
	c.refreshPlaceholders(ctx, false)
}

// onInstanceAdded runs for an instance the hello did not list: fetch its
// started_at (envelope ts stays as the placeholder while that fails), join
// its always rooms and evict downstream host:<inst> clients (vanish +
// reappear is an epoch change).
func (c *Client) onInstanceAdded(ctx context.Context, name, via string) {
	c.logf("xcclient: instance %s appeared (%s)", name, via)
	c.refreshPlaceholders(ctx, true)
	c.joinRooms(alwaysRooms(name))
	c.evict(wire.HostRoomPrefix+":"+name, "instance appeared")
}

// onEpoch handles a per-instance seq regression: the mirror already reset
// the instance; re-read started_at and evict its downstream clients.
func (c *Client) onEpoch(ctx context.Context, name, why string) {
	c.logf("xcclient: %s: %s; epoch change", name, why)
	c.mirror.MarkPlaceholder(name)
	c.refreshPlaceholders(ctx, true)
	c.evict(wire.HostRoomPrefix+":"+name, why)
}

func dedupe(rooms []string) []string {
	seen := make(map[string]bool, len(rooms))
	out := rooms[:0:0]
	for _, r := range rooms {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}
