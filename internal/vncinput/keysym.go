// Package vncinput is the backend-owned counterpart to the admin panel's
// on-screen controller: it injects Xbox-pad input into an xemu container by
// sending RFB KeyEvents over the SAME channel the admin uses.
//
// The admin path (proven working) is:
//
//	XboxController.svelte (button → logical label)
//	  → VNCKeyboard.sendKey  (sveltekit/src/lib/utils/vnc-keyboard.ts)
//	    → RFB KeyEvent over WebSocket
//	      → /api/admin/containers/{name}/vnc  (routes/containers/vnc.go relay)
//	        → ws://127.0.0.1:<BrowserWeb>/websockify   (container's Xvnc)
//	          → xemu reads the keyboard from that Xvnc → keyboard→controller map
//
// This package is the server-side origin of that exact byte stream: it dials
// the container's `/websockify` endpoint directly (the same target vnc.go
// proxies to) and speaks RFB 3.8, so the runner and the admin feed one shared
// keyboard channel — an admin can take over a runner-driven session with
// nothing extra to build.
//
// It exists because neither QMP `sendkey` nor host-side xdotool into a
// `-display xemu` window reaches the controller (see ADR-0003); the container's
// Xvnc is the input surface xemu actually reads, and RFB KeyEvents are how you
// write to it.
//
// The label→keysym table below is a 1:1 mirror of the frontend KEYSYM map
// (sveltekit/src/lib/utils/vnc-keyboard.ts); a test guards that they stay in
// sync. The vpad (internal/vpad) remains for headless *analog* automation
// (movement/aim); this keyboard channel is the digital/menu + admin-shared path.
package vncinput

import "sort"

// Keysym is an X11 keysym value (network byte order on the wire).
type Keysym = uint32

// KEYSYM mirrors sveltekit/src/lib/utils/vnc-keyboard.ts exactly: logical labels
// (the same ones XboxController.svelte and xemu.SendKey use) → X11 keysyms.
var KEYSYM = map[string]Keysym{
	// Face buttons.
	"a": 0x0061, "b": 0x0062, "x": 0x0078, "y": 0x0079,
	// D-pad.
	"Up": 0xff52, "Down": 0xff54, "Left": 0xff51, "Right": 0xff53,
	// Start / Back / Guide.
	"Return": 0xff0d, "BackSpace": 0xff08, "5": 0x0035,
	// Bumpers, L3, R3.
	"1": 0x0031, "2": 0x0032, "3": 0x0033, "4": 0x0034,
	// Triggers.
	"w": 0x0077, "o": 0x006f,
	// Left stick (ESDF).
	"e": 0x0065, "s": 0x0073, "d": 0x0064, "f": 0x0066,
	// Right stick (IJKL).
	"i": 0x0069, "j": 0x006a, "k": 0x006b, "l": 0x006c,
	// Modifier + letter for xemu's Ctrl+R reset chord.
	"Control_L": 0xffe3, "r": 0x0072,
	// Function keys.
	"F1": 0xffbe, "F2": 0xffbf, "F3": 0xffc0, "F4": 0xffc1, "F5": 0xffc2, "F6": 0xffc3,
	"F7": 0xffc4, "F8": 0xffc5, "F9": 0xffc6, "F10": 0xffc7, "F11": 0xffc8, "F12": 0xffc9,
}

// SupportedLabels returns the sorted set of labels the injector accepts.
func SupportedLabels() []string {
	out := make([]string, 0, len(KEYSYM))
	for k := range KEYSYM {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
