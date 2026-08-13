package vncinput

import (
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestKeyEventBytes(t *testing.T) {
	// A button down = the admin's exact bytes: type 4, down 1, 2 pad, keysym BE.
	got := keyEvent(0x0061, true)
	want := []byte{4, 1, 0, 0, 0, 0, 0, 0x61}
	if string(got) != string(want) {
		t.Errorf("keyEvent(a,down) = % x, want % x", got, want)
	}
	got = keyEvent(0xff54, false) // D-pad Down, up
	want = []byte{4, 0, 0, 0, 0, 0, 0xff, 0x54}
	if string(got) != string(want) {
		t.Errorf("keyEvent(Down,up) = % x, want % x", got, want)
	}
}

func TestSetEncodingsBytes(t *testing.T) {
	got := setEncodings(-223, -308, -224)
	// type 2, pad 0, count=3, then three big-endian int32s.
	if got[0] != 2 || got[1] != 0 || binary.BigEndian.Uint16(got[2:]) != 3 {
		t.Fatalf("setEncodings header wrong: % x", got[:4])
	}
	if int32(binary.BigEndian.Uint32(got[4:])) != -223 {
		t.Errorf("first encoding = %d, want -223", int32(binary.BigEndian.Uint32(got[4:])))
	}
}

// TestKeysymMirrorsFrontend guards the label→keysym table against drift from
// sveltekit/src/lib/utils/vnc-keyboard.ts (and XboxController.svelte's buttons).
func TestKeysymMirrorsFrontend(t *testing.T) {
	want := map[string]uint32{
		"a": 0x0061, "b": 0x0062, "x": 0x0078, "y": 0x0079,
		"Up": 0xff52, "Down": 0xff54, "Left": 0xff51, "Right": 0xff53,
		"Return": 0xff0d, "BackSpace": 0xff08, "5": 0x0035,
		"1": 0x0031, "2": 0x0032, "3": 0x0033, "4": 0x0034,
		"w": 0x0077, "o": 0x006f,
		"e": 0x0065, "s": 0x0073, "d": 0x0064, "f": 0x0066,
		"i": 0x0069, "j": 0x006a, "k": 0x006b, "l": 0x006c,
		"Control_L": 0xffe3, "r": 0x0072,
		"F1": 0xffbe, "F2": 0xffbf, "F3": 0xffc0, "F4": 0xffc1, "F5": 0xffc2, "F6": 0xffc3,
		"F7": 0xffc4, "F8": 0xffc5, "F9": 0xffc6, "F10": 0xffc7, "F11": 0xffc8, "F12": 0xffc9,
	}
	if len(KEYSYM) != len(want) {
		t.Errorf("KEYSYM has %d entries, frontend has %d", len(KEYSYM), len(want))
	}
	for label, sym := range want {
		if KEYSYM[label] != sym {
			t.Errorf("KEYSYM[%q] = 0x%x, want 0x%x", label, KEYSYM[label], sym)
		}
	}
}

// mockRFBReader buffers RFB bytes across WebSocket frames on the server side.
type mockRFBReader struct {
	ws  *websocket.Conn
	ctx context.Context
	buf []byte
}

func (r *mockRFBReader) readFull(n int) ([]byte, error) {
	for len(r.buf) < n {
		_, data, err := r.ws.Read(r.ctx)
		if err != nil {
			return nil, err
		}
		r.buf = append(r.buf, data...)
	}
	out := make([]byte, n)
	copy(out, r.buf[:n])
	r.buf = r.buf[n:]
	return out, nil
}

// TestInjectorDrivesRFB spins up a mock RFB-over-WebSocket server (the container
// Xvnc's role), runs the injector's full handshake + Tap("a")/Tap("Down"), and
// asserts the server received the exact KeyEvent bytes the admin panel sends.
// This verifies the injector speaks the admin's channel correctly end-to-end;
// the remaining "does it move the guest" step needs a live container (ADR-0003
// Log).
func TestInjectorDrivesRFB(t *testing.T) {
	keyEvents := make(chan []byte, 16)
	handlerDone := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer close(handlerDone)
		ws, err := websocket.Accept(w, req, &websocket.AcceptOptions{
			Subprotocols:       []string{"binary"},
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		defer ws.Close(websocket.StatusNormalClosure, "")
		ctx := req.Context()
		r := &mockRFBReader{ws: ws, ctx: ctx}
		write := func(b []byte) error { return ws.Write(ctx, websocket.MessageBinary, b) }

		// --- RFB 3.8 server handshake ---
		if write([]byte("RFB 003.008\n")) != nil {
			return
		}
		if _, err := r.readFull(12); err != nil { // client version
			return
		}
		if write([]byte{1, 1}) != nil { // 1 security type: None(1)
			return
		}
		if _, err := r.readFull(1); err != nil { // client selects None
			return
		}
		if write([]byte{0, 0, 0, 0}) != nil { // SecurityResult OK
			return
		}
		if _, err := r.readFull(1); err != nil { // ClientInit shared flag
			return
		}
		serverInit := make([]byte, 24+4)
		binary.BigEndian.PutUint16(serverInit[0:], 1280)
		binary.BigEndian.PutUint16(serverInit[2:], 960)
		binary.BigEndian.PutUint32(serverInit[20:], 4) // name length
		copy(serverInit[24:], "xemu")
		if write(serverInit) != nil {
			return
		}

		// --- read client→server messages, forward KeyEvents ---
		for {
			typ, err := r.readFull(1)
			if err != nil {
				return
			}
			switch typ[0] {
			case 2: // SetEncodings: pad(1) + count(2) + count*4
				rest, err := r.readFull(3)
				if err != nil {
					return
				}
				n := int(binary.BigEndian.Uint16(rest[1:]))
				if _, err := r.readFull(4 * n); err != nil {
					return
				}
			case 4: // KeyEvent: down(1) + pad(2) + keysym(4) = 7 more bytes
				rest, err := r.readFull(7)
				if err != nil {
					return
				}
				ev := append([]byte{4}, rest...)
				select {
				case keyEvents <- ev:
				default:
				}
			default:
				return
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/websockify"

	inj, err := Dial(ctx, wsURL)
	if err != nil {
		t.Fatalf("Dial (handshake): %v", err)
	}
	defer inj.Close()

	if err := inj.Tap("a"); err != nil {
		t.Fatalf("Tap(a): %v", err)
	}
	if err := inj.Tap("Down"); err != nil {
		t.Fatalf("Tap(Down): %v", err)
	}

	// Expect: a-down, a-up, Down-down, Down-up.
	want := [][]byte{
		{4, 1, 0, 0, 0, 0, 0, 0x61},
		{4, 0, 0, 0, 0, 0, 0, 0x61},
		{4, 1, 0, 0, 0, 0, 0xff, 0x54},
		{4, 0, 0, 0, 0, 0, 0xff, 0x54},
	}
	for i, w := range want {
		select {
		case got := <-keyEvents:
			if string(got) != string(w) {
				t.Errorf("KeyEvent[%d] = % x, want % x", i, got, w)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for KeyEvent[%d]", i)
		}
	}
}
