package manager

import (
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/xemu-cartographer/xc-scraper/hostrunner"
	"github.com/xemu-cartographer/xc-scraper/roster"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// Ports (step 7, part 3a). The manager produces wire envelopes and decides
// WHEN to produce them; everything that is league-specific — which WebSocket
// room an envelope lands in, whether anyone is subscribed, which roster rows
// are dummies — arrives through these interfaces. The league server plugs
// its adapters in from internal/leaguescraper; tests plug in stubs.

// Emitter is the outbound transport for every envelope the manager
// broadcasts. instance is the runner name, or "" for cross-instance classes
// (summary). envBytes is the marshalled wire.Envelope — NOT wrapped in a
// wire.Message; the emitter decides the framing and the destination.
type Emitter interface {
	Emit(instance, class string, envBytes []byte)
}

// Demand answers "does anyone want (instance, class) right now?" — the
// subscriber half of shouldRead. A nil Demand is permissive (every class is
// wanted), which is what the runner assumed when no WS hub was attached.
type Demand interface {
	Wants(instance, class string) bool
}

// GameEnd is invoked once per finished match with the wire-shaped record.
// Defined now, wired in part 3b (games_persist still writes through svc.App).
type GameEnd func(fg wire.FinishedGame)

// Options is everything a Manager can be configured with at construction.
// Every field is optional (nil-safe); the existing Set* methods remain the
// same knobs for callers that wire things up after New.
type Options struct {
	// Emitter receives every broadcast envelope. nil → envelopes are dropped
	// (nullEmitter) so call sites never nil-check.
	Emitter Emitter
	// Demand gates the per-poll classes. nil → permissive.
	Demand Demand
	// OnGameEnd fires once per finished match (part 3b).
	OnGameEnd GameEnd
	// RosterFilter resolves the dummy-filter config for an instance. nil →
	// roster.Config{} (no filtering). Consulted behind a 10 s TTL cache.
	RosterFilter func(instance string) roster.Config
	// OffsetSetFor maps an instance to its assigned offset-set id
	// (same knob as SetOffsetSetResolver).
	OffsetSetFor func(instance string) string
	// OverlayFor maps an instance to its overlay qcow2 path
	// (same knob as SetOverlayResolver).
	OverlayFor func(instance string) (string, bool)
	// HostDrive is the host/client scoping gate (same knob as SetHostDrivePolicy).
	HostDrive func(instance string) bool
	// HostURL resolves an instance's websockify URL; HostRegistry is the
	// player-hosting registry. Both together are the SetHostRunner knob;
	// hosting is enabled iff HostRegistry != nil (call SetHostRunner to
	// register without enabling).
	HostURL      func(instance string) (string, bool)
	HostRegistry *hostrunner.Registry
	// Services is TEMPORARY (removed in part 3c): capture_loader,
	// games_persist and hello still read svc.App directly.
	Services *guards.Services
}

// nullEmitter drops everything. Installed when Options.Emitter is nil so the
// runner / aggregator never nil-check the transport.
type nullEmitter struct{}

func (nullEmitter) Emit(string, string, []byte) {}
