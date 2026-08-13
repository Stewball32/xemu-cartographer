package containers

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestDialWithRetrySucceedsAgainstLiveListener: a listener that is already up
// is dialed on the first attempt with no measurable delay.
func TestDialWithRetrySucceedsAgainstLiveListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	dial := newDialWithRetry(2*time.Second, time.Second)
	start := time.Now()
	conn, err := dial(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial live listener: %v", err)
	}
	_ = conn.Close()
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("dial to a live listener took %v; expected near-instant", elapsed)
	}
}

// TestDialWithRetryFailsFastOnDeadPort: a refused (dead) port returns an error
// within roughly the configured budget instead of hanging. Uses a short budget
// so the test stays fast; asserts the retry loop honors the deadline.
func TestDialWithRetryFailsFastOnDeadPort(t *testing.T) {
	// Grab a free port, then close it so connections are refused.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	deadAddr := ln.Addr().String()
	_ = ln.Close()

	const budget = 300 * time.Millisecond
	dial := newDialWithRetry(budget, 100*time.Millisecond)
	start := time.Now()
	conn, err := dial(context.Background(), "tcp", deadAddr)
	elapsed := time.Since(start)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected error dialing a dead port, got nil")
	}
	// Should return at ~budget, not hang. Allow generous slack for slow CI.
	if elapsed > budget+2*time.Second {
		t.Errorf("dial to dead port took %v; expected to bail near budget %v", elapsed, budget)
	}
}

// TestDialWithRetryRespectsCanceledContext: a canceled context aborts the retry
// loop promptly rather than waiting out the budget.
func TestDialWithRetryRespectsCanceledContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	deadAddr := ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dial := newDialWithRetry(10*time.Second, time.Second)
	start := time.Now()
	if _, err := dial(ctx, "tcp", deadAddr); err == nil {
		t.Fatal("expected error with canceled context")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("canceled dial took %v; expected prompt return", elapsed)
	}
}
