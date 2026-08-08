package vncinput

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Zero the focus-settle sleeps and disable the stale-refocus timer so the pump
// tests run fast and deterministically (no surprise re-focus between commands).
func init() {
	focusSettleDelay = 0
	focusReassertDelay = 0
	refocusInterval = time.Hour
}

// fakeDriver records the calls the pump makes and can be told to fail the next
// Tap so reconnect behaviour is exercisable.
type fakeDriver struct {
	mu       sync.Mutex
	calls    []string
	failNext bool
	closed   bool
	events   chan string
}

func newFakeDriver(events chan string) *fakeDriver {
	return &fakeDriver{events: events}
}

func (d *fakeDriver) record(s string) {
	d.mu.Lock()
	d.calls = append(d.calls, s)
	d.mu.Unlock()
	if d.events != nil {
		d.events <- s
	}
}

func (d *fakeDriver) Tap(label string) error {
	d.mu.Lock()
	fail := d.failNext
	d.failNext = false
	d.mu.Unlock()
	if fail {
		d.record("tap-fail:" + label)
		return fmt.Errorf("boom")
	}
	d.record("tap:" + label)
	return nil
}

func (d *fakeDriver) Chord(labels ...string) error {
	d.record("chord:" + fmt.Sprint(labels))
	return nil
}

func (d *fakeDriver) FocusClick() error {
	d.record("focus")
	return nil
}

func (d *fakeDriver) Close() error {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
	return nil
}

func recv(t *testing.T, ch chan string) string {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pump call")
		return ""
	}
}

// A tap dials, focus-clicks + RE-ASSERTS (settle), then taps — in that order.
func TestPumpDialsFocusesThenTaps(t *testing.T) {
	events := make(chan string, 8)
	var dials int
	dial := func(ctx context.Context, url string) (Driver, error) {
		dials++
		return newFakeDriver(events), nil
	}
	p := newPump(context.Background(), "ws://x/websockify", dial, true)
	defer p.Close()

	if err := p.Tap("y"); err != nil {
		t.Fatalf("Tap: %v", err)
	}
	// settleFocus double-clicks (grab + re-assert) before the first key lands.
	if got := recv(t, events); got != "focus" {
		t.Fatalf("first call should be focus, got %q", got)
	}
	if got := recv(t, events); got != "focus" {
		t.Fatalf("second call should be the focus re-assert, got %q", got)
	}
	if got := recv(t, events); got != "tap:y" {
		t.Fatalf("third call should be tap:y, got %q", got)
	}
	if dials != 1 {
		t.Fatalf("expected 1 dial, got %d", dials)
	}
}

// focus=false suppresses the focus click.
func TestPumpNoFocus(t *testing.T) {
	events := make(chan string, 8)
	dial := func(ctx context.Context, url string) (Driver, error) { return newFakeDriver(events), nil }
	p := newPump(context.Background(), "ws://x/websockify", dial, false)
	defer p.Close()

	if err := p.Tap("a"); err != nil {
		t.Fatalf("Tap: %v", err)
	}
	if got := recv(t, events); got != "tap:a" {
		t.Fatalf("with focus off first call should be tap:a, got %q", got)
	}
}

// A write error closes the connection and the next command re-dials.
func TestPumpReconnectsOnError(t *testing.T) {
	events := make(chan string, 16)
	var mu sync.Mutex
	var dials int
	var drivers []*fakeDriver
	dial := func(ctx context.Context, url string) (Driver, error) {
		mu.Lock()
		dials++
		d := newFakeDriver(events)
		if dials == 1 {
			d.failNext = true // first connection's first tap fails
		}
		drivers = append(drivers, d)
		mu.Unlock()
		return d, nil
	}
	p := newPump(context.Background(), "ws://x/websockify", dial, false)
	defer p.Close()

	// First tap fails → connection torn down.
	if err := p.Tap("y"); err != nil {
		t.Fatalf("Tap: %v", err)
	}
	if got := recv(t, events); got != "tap-fail:y" {
		t.Fatalf("expected tap-fail:y, got %q", got)
	}

	// The pump backs off ~2s after a failure before re-dialing; wait it out, then
	// the next command re-dials a fresh driver and succeeds.
	time.Sleep(dialBackoff + 200*time.Millisecond)
	if err := p.Tap("a"); err != nil {
		t.Fatalf("Tap: %v", err)
	}
	if got := recv(t, events); got != "tap:a" {
		t.Fatalf("expected tap:a after reconnect, got %q", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if dials != 2 {
		t.Fatalf("expected 2 dials (initial + reconnect), got %d", dials)
	}
	if !drivers[0].closed {
		t.Fatal("first driver should be closed after the write error")
	}
}

// Close tears down the live connection.
func TestPumpCloseClosesConn(t *testing.T) {
	events := make(chan string, 8)
	var d *fakeDriver
	dial := func(ctx context.Context, url string) (Driver, error) {
		d = newFakeDriver(events)
		return d, nil
	}
	p := newPump(context.Background(), "ws://x/websockify", dial, false)
	if err := p.Tap("a"); err != nil {
		t.Fatalf("Tap: %v", err)
	}
	recv(t, events) // tap:a
	p.Close()
	if d == nil || !d.closed {
		t.Fatal("Close should have closed the live driver")
	}
	// Enqueue after close is rejected, not a panic.
	if err := p.Tap("a"); err == nil {
		t.Fatal("Tap after Close should error")
	}
}

// A full queue returns an error rather than blocking the caller (the scraper
// loop must never block on input).
func TestPumpQueueFull(t *testing.T) {
	// A dial that blocks forever keeps the single in-flight command stuck in the
	// pump goroutine while the buffer fills.
	block := make(chan struct{})
	dial := func(ctx context.Context, url string) (Driver, error) {
		<-block
		return nil, fmt.Errorf("never")
	}
	p := newPump(context.Background(), "ws://x/websockify", dial, false)
	defer func() { close(block); p.Close() }()

	// One command is pulled by the goroutine (stuck in dial); fill the rest of
	// the buffer, then the next enqueue must fail fast.
	filled := 0
	var lastErr error
	for i := 0; i < pumpQueueCap+8; i++ {
		if err := p.Tap("a"); err != nil {
			lastErr = err
			break
		}
		filled++
	}
	if lastErr == nil {
		t.Fatalf("expected a queue-full error after ~%d enqueues, filled=%d", pumpQueueCap, filled)
	}
}
