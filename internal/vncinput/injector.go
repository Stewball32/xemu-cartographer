package vncinput

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// defaultHold mirrors VNCKeyboard.sendChord's 60ms hold and xemu.SendKey's
// default press duration closely enough for menu input.
const defaultHold = 80 * time.Millisecond

// Injector holds one RFB-over-WebSocket connection to a container's Xvnc and
// writes KeyEvents. Not safe for concurrent use — serialise from one goroutine
// (the runner's input loop), like the admin's single VNCKeyboard.
type Injector struct {
	ws     *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	buf    []byte // leftover RFB bytes spanning WebSocket frames
}

// Dial connects to a container's websockify endpoint (e.g.
// "ws://127.0.0.1:3103/websockify" — the same URL vnc.go proxies to), performs
// the RFB 3.8 handshake as a read-only-ish keyboard client (shared flag so it
// coexists with the display connection), and returns a ready Injector.
//
// The parent context bounds the whole connection lifetime; Close cancels it.
func Dial(parent context.Context, wsURL string) (*Injector, error) {
	ctx, cancel := context.WithCancel(parent)
	ws, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Subprotocols: []string{"binary"},
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("vncinput dial %s: %w", wsURL, err)
	}
	// The RFB handshake messages are small but can be gigantic reads for a
	// framebuffer; we never request framebuffer data, so a modest read limit
	// keeps a misbehaving peer from ballooning memory.
	ws.SetReadLimit(1 << 20)

	inj := &Injector{ws: ws, ctx: ctx, cancel: cancel}
	if err := inj.handshake(); err != nil {
		_ = ws.Close(websocket.StatusProtocolError, "handshake failed")
		cancel()
		return nil, fmt.Errorf("vncinput handshake: %w", err)
	}
	return inj, nil
}

// handshake runs the RFB 3.8 client handshake, byte-for-byte matching what
// VNCKeyboard does: version → select security None → SecurityResult → ClientInit
// (shared) → consume ServerInit → advertise DesktopSize encodings.
func (i *Injector) handshake() error {
	// 1. ProtocolVersion: server sends "RFB 003.008\n" (12 bytes); reply same.
	ver, err := i.readFull(12)
	if err != nil {
		return fmt.Errorf("read version: %w", err)
	}
	if !strings.HasPrefix(string(ver), "RFB ") {
		return fmt.Errorf("not an RFB server: %q", string(ver))
	}
	if err := i.write([]byte("RFB 003.008\n")); err != nil {
		return err
	}

	// 2. Security types: count byte + that many type bytes. Require None (1).
	cnt, err := i.readFull(1)
	if err != nil {
		return fmt.Errorf("read security count: %w", err)
	}
	if cnt[0] == 0 {
		return fmt.Errorf("server refused connection during security phase")
	}
	types, err := i.readFull(int(cnt[0]))
	if err != nil {
		return fmt.Errorf("read security types: %w", err)
	}
	hasNone := false
	for _, t := range types {
		if t == 1 { // None
			hasNone = true
		}
	}
	if !hasNone {
		return fmt.Errorf("server requires auth (no 'None' security type)")
	}
	if err := i.write([]byte{1}); err != nil { // select None
		return err
	}

	// 3. SecurityResult: u32, 0 = OK.
	res, err := i.readFull(4)
	if err != nil {
		return fmt.Errorf("read security result: %w", err)
	}
	if binary.BigEndian.Uint32(res) != 0 {
		return fmt.Errorf("security handshake failed (result != 0)")
	}

	// 4. ClientInit: shared-flag = 1 so we coexist with the display connection.
	if err := i.write([]byte{1}); err != nil {
		return err
	}

	// 5. ServerInit: width(2) height(2) pixel-format(16) name-length(4) name(N).
	head, err := i.readFull(24)
	if err != nil {
		return fmt.Errorf("read server-init: %w", err)
	}
	nameLen := binary.BigEndian.Uint32(head[20:24])
	if nameLen > 0 {
		if _, err := i.readFull(int(nameLen)); err != nil {
			return fmt.Errorf("read server-init name: %w", err)
		}
	}

	// 6. SetEncodings advertising DesktopSize / ExtendedDesktopSize / LastRect,
	// matching VNCKeyboard so Xvnc doesn't kick us on framebuffer renegotiation.
	if err := i.write(setEncodings(-223, -308, -224)); err != nil {
		return err
	}
	return nil
}

// SendKeysym sends one RFB KeyEvent (press if down, release otherwise).
func (i *Injector) SendKeysym(keysym Keysym, down bool) error {
	return i.write(keyEvent(keysym, down))
}

// SendKey sends a KeyEvent for a logical label (see KEYSYM / SupportedLabels).
func (i *Injector) SendKey(label string, down bool) error {
	sym, ok := KEYSYM[label]
	if !ok {
		return fmt.Errorf("unknown key %q (see vncinput.SupportedLabels)", label)
	}
	return i.SendKeysym(sym, down)
}

// Tap presses and releases one labelled key using the default hold.
func (i *Injector) Tap(label string) error { return i.TapHold(label, defaultHold) }

// TapHold presses a labelled key, holds it for d, then releases.
func (i *Injector) TapHold(label string, d time.Duration) error {
	if err := i.SendKey(label, true); err != nil {
		return err
	}
	time.Sleep(d)
	return i.SendKey(label, false)
}

// Chord presses several labelled keys together for the default hold, then
// releases them in reverse order (e.g. "Control_L","r" → xemu's soft reset).
func (i *Injector) Chord(labels ...string) error { return i.ChordHold(defaultHold, labels...) }

// ChordHold presses labels together, holds for d, then releases in reverse.
func (i *Injector) ChordHold(d time.Duration, labels ...string) error {
	if len(labels) == 0 {
		return fmt.Errorf("no labels")
	}
	for _, l := range labels {
		if _, ok := KEYSYM[l]; !ok {
			return fmt.Errorf("unknown key %q", l)
		}
	}
	for _, l := range labels {
		if err := i.SendKey(l, true); err != nil {
			return err
		}
	}
	time.Sleep(d)
	var firstErr error
	for j := len(labels) - 1; j >= 0; j-- {
		if err := i.SendKey(labels[j], false); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// SendPointer sends one RFB PointerEvent (§7.5.5): message-type=5, button-mask,
// then x/y as big-endian u16s in framebuffer coordinates. The admin's noVNC
// canvas focuses xemu's Selkies view with a pointer event; the injector is
// otherwise KeyEvent-only, so this exists solely to reproduce that focus grab —
// Selkies forwards keys only when its canvas has DOM focus (ADR-0003 Log).
func (i *Injector) SendPointer(x, y uint16, buttonMask uint8) error {
	return i.write(pointerEvent(buttonMask, x, y))
}

// FocusClick taps the left mouse button at a benign top-left coordinate to give
// the container's Selkies canvas DOM focus so subsequent KeyEvents register.
// Best-effort: coordinate (4,4) is the empty screen corner in CE menus, so the
// click can't activate a centered menu control. Called once by the input pump on
// (re)connect (see pump.go).
func (i *Injector) FocusClick() error {
	if err := i.SendPointer(4, 4, 0x01); err != nil { // button 1 down
		return err
	}
	time.Sleep(20 * time.Millisecond)
	return i.SendPointer(4, 4, 0x00) // release
}

// Close releases all held keys best-effort, then closes the connection.
func (i *Injector) Close() error {
	err := i.ws.Close(websocket.StatusNormalClosure, "")
	i.cancel()
	return err
}

// ---- RFB wire helpers ------------------------------------------------------

// keyEvent builds an RFB KeyEvent message (§7.5.4): type=4, down-flag, 2 pad
// bytes, then the keysym as a big-endian u32. Byte-identical to
// VNCKeyboard.sendKey.
func keyEvent(keysym Keysym, down bool) []byte {
	b := make([]byte, 8)
	b[0] = 4
	if down {
		b[1] = 1
	}
	binary.BigEndian.PutUint32(b[4:], keysym)
	return b
}

// pointerEvent builds an RFB PointerEvent message (§7.5.5): type=5, button-mask,
// x-position (BE u16), y-position (BE u16).
func pointerEvent(buttonMask uint8, x, y uint16) []byte {
	b := make([]byte, 6)
	b[0] = 5
	b[1] = buttonMask
	binary.BigEndian.PutUint16(b[2:], x)
	binary.BigEndian.PutUint16(b[4:], y)
	return b
}

// setEncodings builds an RFB SetEncodings message (§7.5.2).
func setEncodings(encs ...int32) []byte {
	b := make([]byte, 4+4*len(encs))
	b[0] = 2 // SetEncodings
	binary.BigEndian.PutUint16(b[2:], uint16(len(encs)))
	for k, e := range encs {
		binary.BigEndian.PutUint32(b[4+4*k:], uint32(e))
	}
	return b
}

func (i *Injector) write(b []byte) error {
	return i.ws.Write(i.ctx, websocket.MessageBinary, b)
}

// readFull returns exactly n bytes from the RFB stream, buffering across
// WebSocket frames (websockify does not preserve RFB message boundaries).
func (i *Injector) readFull(n int) ([]byte, error) {
	for len(i.buf) < n {
		_, data, err := i.ws.Read(i.ctx)
		if err != nil {
			return nil, err
		}
		i.buf = append(i.buf, data...)
	}
	out := make([]byte, n)
	copy(out, i.buf[:n])
	i.buf = i.buf[n:]
	return out, nil
}
