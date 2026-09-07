package xemu

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Every QMP exchange must be deadline-bounded: a wedged xemu (guest hung with
// the monitor unresponsive) previously blocked the caller forever — including
// RefreshLowHVA on the Idle poll path, which wedged the whole runner.

// serveQMP listens on a unix socket in a temp dir and runs handler on the
// first accepted connection. Returns the socket path.
func serveQMP(t *testing.T, handler func(net.Conn)) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "qmp.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen %s: %v", sock, err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		handler(conn)
	}()
	return sock
}

// handshake replies to the client's greeting + qmp_capabilities exchange.
// Returns false if the client vanished first.
func handshake(conn net.Conn, br *bufio.Reader) bool {
	fmt.Fprintln(conn, `{"QMP":{"version":{},"capabilities":[]}}`)
	if _, err := br.ReadString('\n'); err != nil { // qmp_capabilities
		return false
	}
	fmt.Fprintln(conn, `{"return":{}}`)
	return true
}

// A socket that accepts but never speaks QMP must fail the handshake within
// the command deadline, not hang newQMPClient forever.
func TestNewQMPClientTimesOutOnSilentServer(t *testing.T) {
	sock := serveQMP(t, func(conn net.Conn) {
		_, _ = io.Copy(io.Discard, conn) // read and never reply
	})
	start := time.Now()
	c, err := newQMPClient(sock, qmpTimeouts{cmd: 200 * time.Millisecond})
	if err == nil {
		c.close()
		t.Fatal("expected handshake to time out against a silent server")
	}
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Errorf("err = %v, want os.ErrDeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("handshake took %v, want ~200ms deadline", elapsed)
	}
}

// The key regression test: a server that handshakes then goes mute — the
// wedged-monitor case — must error each command out within the deadline.
func TestHMPTimesOutOnMuteServer(t *testing.T) {
	sock := serveQMP(t, func(conn net.Conn) {
		br := bufio.NewReader(conn)
		if !handshake(conn, br) {
			return
		}
		_, _ = io.Copy(io.Discard, conn) // swallow commands, never reply
	})
	c, err := newQMPClient(sock, qmpTimeouts{cmd: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	defer c.close()
	start := time.Now()
	_, err = c.hmp("gpa2hva 0x0")
	if err == nil {
		t.Fatal("expected hmp to time out against a mute monitor")
	}
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Errorf("err = %v, want os.ErrDeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("hmp took %v, want ~200ms deadline", elapsed)
	}
}

// A slow-but-responsive server must not be affected: the deadline is armed
// per exchange, so back-to-back commands on one connection keep working even
// when their total runtime exceeds one window. Each reply is delayed past
// half the window, so any two consecutive commands overrun a deadline that
// was armed once and never re-armed — the handshake's, or the first
// command's. (With a fresh deadline per command each one finishes with
// margin.)
func TestHMPDeadlineResetsBetweenCommands(t *testing.T) {
	const (
		window     = 400 * time.Millisecond
		replyDelay = 250 * time.Millisecond
		commands   = 3
	)
	sock := serveQMP(t, func(conn net.Conn) {
		br := bufio.NewReader(conn)
		if !handshake(conn, br) {
			return
		}
		for {
			if _, err := br.ReadString('\n'); err != nil {
				return
			}
			time.Sleep(replyDelay)
			fmt.Fprintln(conn, `{"return":" pong "}`)
		}
	})
	c, err := newQMPClient(sock, qmpTimeouts{cmd: window})
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	defer c.close()
	start := time.Now()
	for i := 0; i < commands; i++ {
		got, err := c.hmp("info status")
		if err != nil {
			t.Fatalf("command %d (%v in): %v", i, time.Since(start), err)
		}
		if got != "pong" {
			t.Fatalf("command %d = %q, want %q (trimmed)", i, got, "pong")
		}
	}
	// Guard the premise: the run must have outlasted a single window, or the
	// test proved nothing about re-arming.
	if elapsed := time.Since(start); elapsed < commands*replyDelay || elapsed < window {
		t.Fatalf("%d commands took %v, expected ≥ %v — server delay not in effect",
			commands, elapsed, commands*replyDelay)
	}
}

// Zero/negative fields resolve to the package defaults; explicit values win.
func TestQMPTimeoutsWithDefaults(t *testing.T) {
	cases := []struct {
		name     string
		in       qmpTimeouts
		wantDial time.Duration
		wantCmd  time.Duration
	}{
		{"zero", qmpTimeouts{}, defaultQMPDialTimeout, defaultQMPCmdTimeout},
		{"negative", qmpTimeouts{dial: -1, cmd: -1}, defaultQMPDialTimeout, defaultQMPCmdTimeout},
		{"explicit", qmpTimeouts{dial: time.Second, cmd: 2 * time.Second}, time.Second, 2 * time.Second},
		{"partial", qmpTimeouts{cmd: 10 * time.Second}, defaultQMPDialTimeout, 10 * time.Second},
	}
	for _, c := range cases {
		got := c.in.withDefaults()
		if got.dial != c.wantDial || got.cmd != c.wantCmd {
			t.Errorf("%s: withDefaults() = {dial:%v cmd:%v}, want {dial:%v cmd:%v}",
				c.name, got.dial, got.cmd, c.wantDial, c.wantCmd)
		}
	}
}

// Instance override fields plumb through to the client's timeouts.
func TestInstanceQMPTimeoutOverride(t *testing.T) {
	inst := &Instance{QMPDialTimeout: time.Second, QMPCommandTimeout: 30 * time.Second}
	got := inst.qmpTimeouts().withDefaults()
	if got.dial != time.Second || got.cmd != 30*time.Second {
		t.Errorf("qmpTimeouts() = {dial:%v cmd:%v}, want overrides preserved", got.dial, got.cmd)
	}
	got = (&Instance{}).qmpTimeouts().withDefaults()
	if got.dial != defaultQMPDialTimeout || got.cmd != defaultQMPCmdTimeout {
		t.Errorf("zero-value Instance = {dial:%v cmd:%v}, want package defaults", got.dial, got.cmd)
	}
}
