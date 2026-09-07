package manager

import "github.com/xemu-cartographer/xc-scraper/wire"

// Phase is the runner's lifecycle state. Every per-instance runner moves
// through this state machine in response to xemu memory observations:
//
//	Idle  → Ready : XBE title-ID becomes recognised, GameReader is bound.
//	Ready → Live  : ReadGameState observes in_game.
//	Live  → Ready : ReadGameState leaves in_game (deferred so a panic /
//	                ctx-cancel mid-match still moves the just-ended match
//	                into the cache's PreviousGame slot).
//	Ready → Idle  : title-ID re-check fails or returns an unrecognised id;
//	                reader is dropped, runner goes back to title polling.
//	Live  → Idle  : ReadGameState errors for ConsecutiveReadFailureLimit
//	                consecutive polls (heartbeat fallback for the case xemu
//	                exits or the user quits to the dashboard mid-match —
//	                see ROADMAP M5 OQ6).
//
// Alias of wire.Phase — the value is published on the wire (game.phase,
// summary.hosts[].phase, events reply phase), so the type lives in the
// contract package and the constants are re-declared here.
type Phase = wire.Phase

const (
	PhaseIdle  = wire.PhaseIdle
	PhaseReady = wire.PhaseReady
	PhaseLive  = wire.PhaseLive
)
