package vncinput

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Driver is the subset of *Injector the Pump drives. Keeping it an interface is
// what lets the Pump be unit-tested against a fake connection with no live Xvnc.
type Driver interface {
	Tap(label string) error
	Chord(labels ...string) error
	FocusClick() error
	Close() error
}

// DialFunc opens a Driver to a websockify URL. The default (DefaultDial) is a
// thin wrapper over Dial; tests inject a fake so the Pump can be exercised with
// no container.
type DialFunc func(ctx context.Context, url string) (Driver, error)

// DefaultDial dials a real container Xvnc via Dial.
func DefaultDial(ctx context.Context, url string) (Driver, error) {
	inj, err := Dial(ctx, url)
	if err != nil {
		return nil, err // avoid a non-nil interface wrapping a nil *Injector
	}
	return inj, nil
}

// dialBackoff bounds how fast the pump re-dials after a failed connect / a write
// error, so a container whose Xvnc isn't up yet (Firefox still booting) doesn't
// get hammered every tick.
const dialBackoff = 2 * time.Second

// pumpQueueCap is the command buffer depth. The runner emits at most one key per
// tick and gates re-presses on observed state, so a small buffer is plenty; an
// over-cap enqueue returns an error the runner logs rather than blocking the
// scraper loop.
const pumpQueueCap = 16

// Focus-settle timing (package vars so tests can zero them). A KeyEvent sent before
// the browser has moved DOM focus onto the Selkies canvas is DROPPED — the root cause
// of BOTH the cold-boot Down-loop (wake presses lost → dela stays empty → loop) and
// the System Link lost-first-A. CRUCIALLY, on a COLD boot Xvnc accepts our RFB
// connection (dial succeeds) BEFORE the viewer's Firefox/Selkies canvas is up to
// forward keys — so a one-time settle on connect isn't enough; the canvas may come up
// seconds later. So we (re)grab + SETTLE focus before EVERY key during a warmup
// window, and whenever focus goes stale after it — and always WAIT after the click
// before the key, so as soon as the canvas is live a press lands.
var (
	// focusSettleDelay: wait after a focus click for the browser to focus the canvas
	// BEFORE sending the key. (The old code sent the key ~20ms after the click.)
	focusSettleDelay = 300 * time.Millisecond
	// focusWarmup: after (re)connect, re-grab + settle focus before EVERY key for this
	// long — the cold-boot window where the viewer canvas may still be coming up.
	// Generous because a freshly-booted box's Firefox/Selkies can lag well behind Xvnc.
	focusWarmup = 30 * time.Second
	// refocusInterval: after warmup, re-grab focus before a key only when this long has
	// passed since the last click — recovers a dropped focus without clicking every key.
	refocusInterval = 2 * time.Second
)

// Pump is the async write side that connects the state-aware runner (which ticks
// on the scraper loop goroutine and must never block on network I/O) to a
// container's Xvnc. It owns one Injector on its own goroutine, dialing lazily and
// reconnecting on failure, and serialises every key through that single
// goroutine — satisfying the Injector's "not safe for concurrent use" contract
// while keeping Tap/Chord non-blocking for the caller.
//
// *Pump satisfies hostrunner.Input (Tap/Chord), so the manager passes it straight
// in as the runner's input channel.
type Pump struct {
	url   string
	dial  DialFunc
	focus bool

	cmds   chan pumpCmd
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	mu        sync.Mutex
	connected bool // last-known connection state, for Connected()
}

type pumpCmd struct {
	chord bool
	keys  []string
}

// NewPump starts a pump targeting url (e.g. "ws://127.0.0.1:3103/websockify").
// The goroutine runs until Close or parent cancellation. focus-click on connect
// is enabled (the Selkies focus grab, ADR-0003 Log).
func NewPump(parent context.Context, url string) *Pump {
	return newPump(parent, url, DefaultDial, true)
}

func newPump(parent context.Context, url string, dial DialFunc, focus bool) *Pump {
	ctx, cancel := context.WithCancel(parent)
	p := &Pump{
		url:    url,
		dial:   dial,
		focus:  focus,
		cmds:   make(chan pumpCmd, pumpQueueCap),
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go p.run()
	return p
}

// Tap enqueues a single labelled key press+release. Non-blocking: returns an
// error if the pump is closed or its queue is full (the runner logs it and
// re-emits on a later tick).
func (p *Pump) Tap(label string) error { return p.enqueue(pumpCmd{keys: []string{label}}) }

// Chord enqueues a simultaneous multi-key press. Non-blocking, same error
// semantics as Tap.
func (p *Pump) Chord(labels ...string) error { return p.enqueue(pumpCmd{chord: true, keys: labels}) }

func (p *Pump) enqueue(cmd pumpCmd) error {
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("vncinput pump closed")
	default:
	}
	select {
	case p.cmds <- cmd:
		return nil
	default:
		return fmt.Errorf("vncinput pump queue full (%s)", p.url)
	}
}

// Connected reports whether the pump currently holds a live connection. Advisory
// only (for status surfaces) — the value can go stale the instant it's read.
func (p *Pump) Connected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.connected
}

func (p *Pump) setConnected(v bool) {
	p.mu.Lock()
	p.connected = v
	p.mu.Unlock()
}

func (p *Pump) run() {
	defer close(p.done)

	var conn Driver
	var lastFail time.Time
	var lastFocus time.Time
	var connectedAt time.Time
	closeConn := func() {
		if conn != nil {
			_ = conn.Close()
			conn = nil
			p.setConnected(false)
			lastFocus = time.Time{}
		}
	}
	defer closeConn()

	for {
		select {
		case <-p.ctx.Done():
			return
		case cmd := <-p.cmds:
			if conn == nil {
				// Back off after a failure so a not-yet-ready Xvnc isn't hammered.
				// The very first attempt (lastFail zero) always dials.
				if !lastFail.IsZero() && time.Since(lastFail) < dialBackoff {
					continue // drop this command; the runner re-emits later
				}
				c, err := p.dial(p.ctx, p.url)
				if err != nil {
					lastFail = time.Now()
					log.Printf("vncinput pump: dial %s: %v", p.url, err)
					continue
				}
				conn = c
				connectedAt = time.Now()
				lastFocus = time.Time{} // force a focus+settle before the first key
				lastFail = time.Time{}
				p.setConnected(true)
			}
			// Grab the Selkies canvas + SETTLE before the key when needed: the first key
			// after connect; EVERY key during the cold-boot warmup (Xvnc accepted our RFB
			// connection but the viewer's Firefox/Selkies canvas may still be coming up,
			// so earlier clicks/keys land nowhere); or when focus has gone stale. The
			// WAIT after the click is what makes the key land — so as soon as the canvas
			// is live a press registers, dela populates, and the runner (which won't
			// advance until it observes that state change) proceeds. b008fc8 settled only
			// on the initial connect; the recovery path (periodic re-focus) still sent the
			// key ~20ms after the click and dropped it on a not-yet-ready viewer.
			if p.focus && (lastFocus.IsZero() ||
				time.Since(connectedAt) < focusWarmup ||
				time.Since(lastFocus) > refocusInterval) {
				if err := conn.FocusClick(); err != nil {
					log.Printf("vncinput pump: focus %s: %v", p.url, err)
				}
				if focusSettleDelay > 0 {
					time.Sleep(focusSettleDelay)
				}
				lastFocus = time.Now()
			}
			if err := p.exec(conn, cmd); err != nil {
				log.Printf("vncinput pump: exec %s: %v", p.url, err)
				closeConn()
				lastFail = time.Now()
			}
		}
	}
}

func (p *Pump) exec(conn Driver, cmd pumpCmd) error {
	if cmd.chord {
		return conn.Chord(cmd.keys...)
	}
	if len(cmd.keys) == 1 {
		return conn.Tap(cmd.keys[0])
	}
	return nil
}

// Close stops the pump goroutine and releases its connection. Idempotent; blocks
// until the goroutine exits so the caller can rely on the connection being torn
// down on return.
func (p *Pump) Close() error {
	p.cancel()
	<-p.done
	return nil
}
