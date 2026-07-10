package hostrunner

import "sync"

// Authority is the arbitration state (requirement 4). Because the runner and the
// admin drive the SAME channel (internal/vncinput = the admin's exact RFB path),
// takeover is native: the runner just stands down while the admin's keysyms flow.
type Authority int

const (
	// AuthRunner: the runner drives (emits input).
	AuthRunner Authority = iota
	// AuthAdmin: an admin has taken control — the runner is suspended so its
	// presses don't fight the admin's keysyms on the shared channel.
	AuthAdmin
	// AuthDisabled: auto-hosting is off entirely.
	AuthDisabled
)

func (a Authority) String() string {
	switch a {
	case AuthAdmin:
		return "admin"
	case AuthDisabled:
		return "disabled"
	default:
		return "runner"
	}
}

// Arbiter holds the authority for one instance's runner. Safe for concurrent use
// so control endpoints (a request goroutine) and the runner tick (the scraper
// loop goroutine) can share it.
type Arbiter struct {
	mu   sync.RWMutex
	auth Authority
}

// NewArbiter starts runner-driven.
func NewArbiter() *Arbiter { return &Arbiter{auth: AuthRunner} }

// Authority returns the current state.
func (a *Arbiter) Authority() Authority {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.auth
}

// CanEmit reports whether the runner may emit input this tick (only when
// runner-driven).
func (a *Arbiter) CanEmit() bool { return a.Authority() == AuthRunner }

// TakeOver: an admin takes control. No-op when disabled (an admin can drive the
// channel manually regardless, but the runner-arbiter stays disabled so Release
// doesn't silently re-enable auto-hosting).
func (a *Arbiter) TakeOver() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.auth != AuthDisabled {
		a.auth = AuthAdmin
	}
}

// Release: hand control back to the runner (only from admin-override; a disabled
// runner must be Enabled first).
func (a *Arbiter) Release() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.auth == AuthAdmin {
		a.auth = AuthRunner
	}
}

// Disable turns auto-hosting off from any state.
func (a *Arbiter) Disable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.auth = AuthDisabled
}

// Enable re-enables the runner from the disabled state (back to runner-driven).
func (a *Arbiter) Enable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.auth == AuthDisabled {
		a.auth = AuthRunner
	}
}

// Set forces a specific authority (used by the control endpoint's declarative
// PUT). Any state → any state.
func (a *Arbiter) Set(auth Authority) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.auth = auth
}
