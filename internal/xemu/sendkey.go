package xemu

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Programmatic keyboard input into a xemu instance via QMP.
//
// This is the headless counterpart to the human VNC-keyboard path
// (sveltekit/src/lib/utils/vnc-keyboard.ts): instead of an RFB KeyEvent that
// reaches xemu through the container's X server, we issue QEMU's HMP `sendkey`
// over the same QMP socket the scraper already uses for memory translation.
//
// Callers name keys with the SAME logical labels the frontend VNC map uses
// (a/b/x/y, Up/Down/Left/Right, Return, BackSpace, digit strings, ESDF/IJKL
// sticks, Control_L, F-keys) so the two input surfaces speak one vocabulary.
// keyToQEMU translates those labels into QEMU `sendkey` keycode names, which
// differ (Return→ret, BackSpace→backspace, Up→up, Control_L→ctrl, …).
//
// IMPORTANT ROUTING CAVEAT (verified live 2026-07-10 against the pad-bound
// ce-nav instance): xemu's keyboard→Xbox-controller mapping is driven by the
// SDL scancode path (keyboard_controller_scancode_map), the same path the
// kiosk's VNC keyboard feeds through the X server. QMP `sendkey` delivers via
// QEMU's qemu_input layer to the emulated i8042/USB keyboard — a DIFFERENT
// channel that xemu's controller emulation does not read, and which requires a
// port bound to 'keyboard' (ce-nav binds virtual gamepads). So this primitive
// issues correctly but is NOT confirmed to drive Xbox pad input; see
// cmd/inputpoc and docs/decisions for the empirical result and the
// uinput-virtual-pad path that does work. The API is still the right home for
// guest-keyboard / monitor-key input and for the day a keyboard-bound instance
// is wired.

// defaultKeyHold mirrors QEMU's built-in `sendkey` default hold time (100ms).
// Used when a caller doesn't specify a hold duration.
const defaultKeyHold = 100 * time.Millisecond

// keyToQEMU maps the frontend VNC-keyboard logical labels to QEMU `sendkey`
// keycode names. Keys mirror sveltekit/src/lib/utils/vnc-keyboard.ts so both
// input surfaces accept the same names.
var keyToQEMU = map[string]string{
	// Face buttons.
	"a": "a", "b": "b", "x": "x", "y": "y",
	// D-pad.
	"Up": "up", "Down": "down", "Left": "left", "Right": "right",
	// Start / Back / Guide.
	"Return": "ret", "BackSpace": "backspace", "5": "5",
	// Bumpers, L3, R3.
	"1": "1", "2": "2", "3": "3", "4": "4",
	// Triggers.
	"w": "w", "o": "o",
	// Left stick (ESDF).
	"e": "e", "s": "s", "d": "d", "f": "f",
	// Right stick (IJKL).
	"i": "i", "j": "j", "k": "k", "l": "l",
	// Modifier + letter for xemu's Ctrl+R reset chord.
	"Control_L": "ctrl", "r": "r",
	// Function keys.
	"F1": "f1", "F2": "f2", "F3": "f3", "F4": "f4", "F5": "f5", "F6": "f6",
	"F7": "f7", "F8": "f8", "F9": "f9", "F10": "f10", "F11": "f11", "F12": "f12",
}

// SupportedKeys returns the sorted set of logical key labels SendKey/SendChord
// accept. Handy for diagnostics and tests.
func SupportedKeys() []string {
	out := make([]string, 0, len(keyToQEMU))
	for k := range keyToQEMU {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// qemuKeyName translates one logical key label to its QEMU keycode name.
func qemuKeyName(name string) (string, error) {
	q, ok := keyToQEMU[name]
	if !ok {
		return "", fmt.Errorf("unknown key %q (see xemu.SupportedKeys)", name)
	}
	return q, nil
}

// qemuChord translates a list of logical key labels into a QEMU `sendkey`
// chord string (keycodes joined by '-'), the form HMP expects for
// simultaneously-held keys, e.g. []{"Control_L","r"} → "ctrl-r".
func qemuChord(keys []string) (string, error) {
	if len(keys) == 0 {
		return "", fmt.Errorf("no keys")
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		q, err := qemuKeyName(k)
		if err != nil {
			return "", err
		}
		parts[i] = q
	}
	return strings.Join(parts, "-"), nil
}

// holdToMs converts a hold duration to the integer milliseconds HMP `sendkey`
// takes as its optional trailing argument. A non-positive duration yields 0,
// which buildSendKeyCmd renders as "omit the argument" so QEMU applies its own
// 100ms default. Sub-millisecond positive durations clamp to 1ms.
func holdToMs(hold time.Duration) int {
	if hold <= 0 {
		return 0
	}
	ms := int(hold / time.Millisecond)
	if ms < 1 {
		ms = 1
	}
	return ms
}

// buildSendKeyCmd renders the HMP command line for a chord + hold. holdMs<=0
// omits the trailing hold argument (QEMU default 100ms).
func buildSendKeyCmd(chord string, holdMs int) string {
	if holdMs > 0 {
		return fmt.Sprintf("sendkey %s %d", chord, holdMs)
	}
	return "sendkey " + chord
}

// SendKey presses and releases a single key using QEMU's default hold time.
// key is a logical label from the frontend VNC map (see SupportedKeys).
func (inst *Instance) SendKey(key string) error {
	return inst.SendChordHold(defaultKeyHold, key)
}

// SendKeyHold presses a single key held for the given duration, then releases.
func (inst *Instance) SendKeyHold(key string, hold time.Duration) error {
	return inst.SendChordHold(hold, key)
}

// SendChord presses a set of keys simultaneously (e.g. "Control_L","r") for the
// default hold time, then releases them, using QEMU `sendkey`'s chord form.
func (inst *Instance) SendChord(keys ...string) error {
	return inst.SendChordHold(defaultKeyHold, keys...)
}

// SendChordHold is the primitive the others build on: translate the logical key
// labels, open a QMP connection, issue HMP `sendkey`, and map the result to an
// error. A fresh connection per call mirrors RefreshLowHVA; a future high-rate
// runner should hold a persistent QMP client instead of reconnecting per key.
func (inst *Instance) SendChordHold(hold time.Duration, keys ...string) error {
	chord, err := qemuChord(keys)
	if err != nil {
		return fmt.Errorf("%s: sendkey: %w", inst.Name, err)
	}
	qmp, err := newQMPClient(inst.QMPSock, inst.qmpTimeouts())
	if err != nil {
		return fmt.Errorf("%s: sendkey QMP: %w", inst.Name, err)
	}
	defer qmp.close()
	if err := qmp.sendKeyRaw(buildSendKeyCmd(chord, holdToMs(hold)), hold); err != nil {
		return fmt.Errorf("%s: %w", inst.Name, err)
	}
	return nil
}
