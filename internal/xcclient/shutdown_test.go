package xcclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// syncBuffer captures the std logger (the xc hub logs through log.Printf)
// while goroutines from the hub may still be writing.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// runClient is bootClient without the cleanup-bound context: the test owns
// cancel and waits for Run to return.
func runClient(t *testing.T, base string, mutate func(*Config), tune func(*Client)) (c *Client, logs *logRecorder, cancel func(), done <-chan struct{}) {
	t.Helper()
	logs = &logRecorder{}
	cfg := Config{URL: base, Token: testToken, Log: logs.Logf, StaleAfter: 30 * time.Second}
	if mutate != nil {
		mutate(&cfg)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c.backoffMin = 20 * time.Millisecond
	c.backoffMax = 40 * time.Millisecond
	c.fetchRetry = 0
	if tune != nil {
		tune(c)
	}
	ctx, cancelCtx := context.WithCancel(context.Background())
	d := make(chan struct{})
	go func() {
		defer close(d)
		_ = c.Run(ctx)
	}()
	t.Cleanup(cancelCtx)
	return c, logs, cancelCtx, d
}

func waitRun(t *testing.T, done <-chan struct{}, within time.Duration) time.Duration {
	t.Helper()
	start := time.Now()
	select {
	case <-done:
	case <-time.After(within):
		t.Fatalf("Run did not stop within %s", within)
	}
	return time.Since(start)
}

// TestShutdownIsCloseHandshake pins the graceful teardown: cancelling Run
// (Wire.Close) sends 1001 going away with CloseReason, so the daemon's hub
// sees a close status instead of logging "hub: read error: EOF".
func TestShutdownIsCloseHandshake(t *testing.T) {
	// Leg 1: the exact close frame, seen by a raw accept.
	status := make(chan websocket.CloseError, 1)
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_, _, err = conn.Read(r.Context())
		var ce websocket.CloseError
		if !errors.As(err, &ce) {
			ce = websocket.CloseError{Code: -1, Reason: err.Error()}
		}
		status <- ce
	}))
	defer raw.Close()

	c, _, cancel, done := runClient(t, raw.URL, nil, nil)
	waitFor(t, "connect", func() bool { return c.Status().Connected })
	cancel()
	waitRun(t, done, 2*time.Second)
	select {
	case ce := <-status:
		if ce.Code != websocket.StatusGoingAway || ce.Reason != CloseReason {
			t.Fatalf("daemon saw close %v %q, want %v %q", ce.Code, ce.Reason, websocket.StatusGoingAway, CloseReason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("daemon never saw the close")
	}
	if c.Status().Connected {
		t.Fatal("still Connected after Run returned")
	}

	// Leg 2: the real xc hub logs nothing for it.
	logs := &syncBuffer{}
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	rp := newFixtureReplayer(t, "smoke1")
	rp.serve(t, "summary")
	up := newUpstream(t, rp)
	c, _, cancel, done = runClient(t, up.url, nil, nil)
	waitFor(t, "hub join", func() bool { return up.hub.RoomHasMembers(wire.SummaryRoom) })
	cancel()
	waitRun(t, done, 2*time.Second)
	waitFor(t, "hub client gone", func() bool { return up.hub.ClientCount() == 0 })
	if out := logs.String(); strings.Contains(out, "hub: read error") {
		t.Fatalf("hub logged a read error on a clean shutdown:\n%s", out)
	}
}

// TestShutdownDropsSocketWhenEchoIsLate pins the closeGrace fallback: a
// peer that never answers the close frame is dropped after the grace, so
// Wire.Close still returns promptly.
func TestShutdownDropsSocketWhenEchoIsLate(t *testing.T) {
	hold := make(chan struct{})
	defer close(hold)
	mute := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		<-hold // never reads, so the close frame is never echoed
	}))
	defer mute.Close()

	c, _, cancel, done := runClient(t, mute.URL, nil, func(c *Client) { c.closeGrace = 50 * time.Millisecond })
	waitFor(t, "connect", func() bool { return c.Status().Connected })
	cancel()
	if took := waitRun(t, done, 2*time.Second); took > time.Second {
		t.Fatalf("Run took %s to stop past a mute peer", took)
	}
}

// TestRejectedFeedTokenSurfaces pins the token-rejection visibility: the
// hub admits a wrong token as anonymous (hello with no instances, forbidden
// on every join), which used to leave Status looking healthy. Now
// AuthRejected + LastError flip, the log says so once per episode (not per
// join, not per reconnect, never printing the token), and the first frame
// through a room clears it again.
func TestRejectedFeedTokenSurfaces(t *testing.T) {
	const wrong = "not-the-feed-token"
	rp := newFixtureReplayer(t, "smoke1")
	rp.serve(t, "summary")
	up := newUpstream(t, rp)
	addr := strings.TrimPrefix(up.url, "http://")

	c, logs := bootClient(t, up.url, func(cfg *Config) { cfg.Token = wrong }, nil)
	waitFor(t, "auth rejected", func() bool { return c.Status().AuthRejected })
	st := c.Status()
	if !st.Connected || st.Stale || len(c.Mirror().Instances()) != 0 {
		t.Fatalf("Status = %+v, want connected + rejected + empty mirror", st)
	}
	if !strings.Contains(st.LastError, "feed token rejected") || !strings.Contains(st.LastError, wire.SummaryRoom) {
		t.Fatalf("LastError = %q", st.LastError)
	}
	if logs.has("upstream error frame") {
		t.Fatal("forbidden logged as a generic error frame")
	}
	if n := logs.count("rejected the feed token"); n != 1 {
		t.Fatalf("rejection logged %d times, want 1", n)
	}
	for _, line := range logs.snapshot() {
		if strings.Contains(line, wrong) {
			t.Fatalf("token leaked into the log: %s", line)
		}
	}

	// Another refused join (a demand room) is the same episode.
	c.Join(wire.HostRoomPrefix + ":smoke1:" + wire.ClassTick)
	time.Sleep(50 * time.Millisecond)
	if n := logs.count("rejected the feed token"); n != 1 {
		t.Fatalf("second join logged again (%d)", n)
	}

	// A daemon restart with the same token: reconnect, still rejected, still one line.
	up.stop()
	waitFor(t, "disconnect", func() bool { return !c.Status().Connected })
	up2 := newUpstreamOn(t, rp, addr, testToken)
	waitFor(t, "reconnect", func() bool { return c.Status().Reconnects == 1 && up2.hub.ClientCount() == 1 })
	time.Sleep(50 * time.Millisecond)
	st = c.Status()
	if !st.AuthRejected || !strings.Contains(st.LastError, "feed token rejected") {
		t.Fatalf("after reconnect Status = %+v, want still rejected", st)
	}
	if n := logs.count("rejected the feed token"); n != 1 {
		t.Fatalf("reconnect logged again (%d)", n)
	}

	// The daemon now accepts the token: the summary replay clears the latch.
	up2.stop()
	waitFor(t, "disconnect", func() bool { return !c.Status().Connected })
	up3 := newUpstreamOn(t, rp, addr, wrong)
	waitFor(t, "accepted", func() bool {
		return c.Status().Reconnects == 2 && !c.Status().AuthRejected && up3.hub.RoomHasMembers(wire.SummaryRoom)
	})
	if st := c.Status(); st.LastError != "" || !st.Connected {
		t.Fatalf("after acceptance Status = %+v", st)
	}
	if !logs.has("accepted the feed token") {
		t.Fatal("recovery not logged")
	}
}

// TestStreamForbiddenLatchesAuthRejected is the fake-daemon leg: a
// forbidden error frame latches AuthRejected; a data frame clears it.
func TestStreamForbiddenLatchesAuthRejected(t *testing.T) {
	d := newFakeDaemon(t)
	c, logs := newTestClient(t, d, nil, nil)
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)

	errFrame, _ := json.Marshal(wire.Message{Type: wire.TypeError, Room: wire.SummaryRoom, Payload: []byte(`{"code":"forbidden","message":"not allowed to join this room"}`)})
	d.push(errFrame)
	waitFor(t, "latched", func() bool { return c.Status().AuthRejected })
	d.push(errFrame)
	d.push(frame(t, "game"))
	waitFor(t, "cleared", func() bool { return !c.Status().AuthRejected })
	if n := logs.count("rejected the feed token"); n != 1 {
		t.Fatalf("rejection logged %d times, want 1", n)
	}
	if st := c.Status(); st.LastError != "" {
		t.Fatalf("LastError = %q after acceptance", st.LastError)
	}
}

// TestStatusCountsAttemptsAndLastError pins the reconnect bookkeeping the
// banner reads: Attempts climbs while the daemon is away and LastError
// names the (redacted) failure; both reset on the next connect.
func TestStatusCountsAttemptsAndLastError(t *testing.T) {
	d := newFakeDaemon(t)
	c, _ := newTestClient(t, d, nil, nil)
	d.waitConnect()
	if st := c.Status(); st.Attempts != 0 || st.LastError != "" {
		t.Fatalf("fresh Status = %+v", st)
	}
	d.setReject(true)
	d.closeConn()
	waitFor(t, "attempts", func() bool { st := c.Status(); return st.Attempts >= 2 && st.LastError != "" })
	if st := c.Status(); strings.Contains(st.LastError, testToken) {
		t.Fatalf("LastError leaks the token: %q", st.LastError)
	}
	d.setReject(false)
	d.waitConnect()
	waitFor(t, "reset", func() bool { st := c.Status(); return st.Connected && st.Attempts == 0 && st.LastError == "" })
}
