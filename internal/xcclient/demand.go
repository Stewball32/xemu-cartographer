package xcclient

import (
	"sort"
	"sync"
	"time"

	"github.com/xemu-cartographer/xc-scraper/wire"
)

// DemandClasses are the demand-gated classes (DESIGN-STEP8 §8.3): their
// host:<inst>:<class> rooms are joined upstream on the downstream 0→1
// occupancy edge and left after a linger on the 1→0 edge. Every other
// per-instance class is an always-class (AlwaysClasses), joined on hello and
// never left.
var DemandClasses = []string{
	wire.ClassTick,
	wire.ClassObjects,
	wire.ClassDebug,
	wire.ClassGameFiltered,
	wire.ClassEventFiltered,
}

// DefaultLinger is how long a demand room stays subscribed upstream after
// its last downstream subscriber left (§8.3: 5 s).
const DefaultLinger = 5 * time.Second

// DemandRoom reports whether room is demand-gated (host:<inst>:<class> with
// class ∈ DemandClasses) and returns its parse.
func DemandRoom(room string) (wire.Room, bool) {
	rm, err := wire.ParseRoom(room)
	if err != nil || !rm.IsHost() || rm.Aggregate || rm.Class == "" {
		return wire.Room{}, false
	}
	for _, class := range DemandClasses {
		if rm.Class == class {
			return rm, true
		}
	}
	return wire.Room{}, false
}

// Demand mirrors downstream room occupancy onto the upstream subscription
// set. Observe has the flagship hub's RoomObserver signature — wire it with
// ws.Hub.SetRoomObserver(d.Observe) before the hub runs. The observer
// recomputes occupancy on every event (F2), so Observe treats each call as
// the current truth: occupied joins (cancelling a pending leave), vacated
// arms a linger timer whose expiry leaves upstream and drops the cached
// frame so a later joiner is served by the upstream replay rather than a
// frozen snapshot. Always-rooms and non-host rooms are ignored.
//
// The join/leave set persists across upstream reconnects through
// Client.Join / Client.Leave (the client re-joins its demand set on every
// hello).
type Demand struct {
	client *Client
	linger time.Duration
	logf   func(string, ...any)

	mu      sync.Mutex
	joined  map[string]bool
	pending map[string]*time.Timer
	closed  bool
}

// NewDemand returns a demand layer over c (must be non-nil). linger <= 0
// means DefaultLinger.
func NewDemand(c *Client, linger time.Duration) *Demand {
	if c == nil {
		panic("xcclient: NewDemand with nil client")
	}
	if linger <= 0 {
		linger = DefaultLinger
	}
	return &Demand{
		client:  c,
		linger:  linger,
		logf:    c.logf,
		joined:  make(map[string]bool),
		pending: make(map[string]*time.Timer),
	}
}

// Observe is the RoomObserver callback: room went occupied (0→1) or vacant
// (1→0) downstream. Safe from any goroutine; never blocks on the hub.
func (d *Demand) Observe(room string, occupied bool) {
	rm, ok := DemandRoom(room)
	if !ok {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	if occupied {
		if t := d.pending[room]; t != nil {
			t.Stop()
			delete(d.pending, room)
		}
		if d.joined[room] {
			return
		}
		d.joined[room] = true
		d.client.Join(room)
		d.logf("xcclient: demand join %s", room)
		return
	}
	if !d.joined[room] || d.pending[room] != nil {
		return
	}
	d.pending[room] = time.AfterFunc(d.linger, func() { d.release(room, rm) })
}

// release runs when a linger timer expires: leave upstream + drop the cached
// frame, unless the room was re-occupied (timer cancelled) meanwhile.
func (d *Demand) release(room string, rm wire.Room) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed || d.pending[room] == nil {
		return
	}
	delete(d.pending, room)
	if !d.joined[room] {
		return
	}
	delete(d.joined, room)
	d.client.Leave(room)
	d.client.Mirror().Drop(rm.Instance, rm.Class)
	d.logf("xcclient: demand leave %s after %s idle", room, d.linger)
}

// Rooms returns the demand rooms currently joined upstream, sorted;
// rooms inside their linger window are still included.
func (d *Demand) Rooms() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, 0, len(d.joined))
	for room := range d.joined {
		out = append(out, room)
	}
	sort.Strings(out)
	return out
}

// Lingering reports whether room is joined but waiting for its linger timer.
func (d *Demand) Lingering(room string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.joined[room] && d.pending[room] != nil
}

// Close cancels every pending linger timer and ignores further events.
// Rooms stay joined upstream: Close is for shutdown, where the upstream
// connection goes away with the process.
func (d *Demand) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	for room, t := range d.pending {
		t.Stop()
		delete(d.pending, room)
	}
}
