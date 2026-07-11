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
	closeConn := func() {
		if conn != nil {
			_ = conn.Close()
			conn = nil
			p.setConnected(false)
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
				lastFail = time.Time{}
				p.setConnected(true)
				if p.focus {
					if err := conn.FocusClick(); err != nil {
						log.Printf("vncinput pump: focus %s: %v", p.url, err)
					}
				}
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
