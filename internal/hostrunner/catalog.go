package hostrunner

// This is the display catalog the player-scoped /api/play/options endpoint
// serves so the play tab can render a map / gametype picker. It is intentionally
// static, curated data — NOT read from guest memory (CE exposes no enumerable
// map/gametype list at a stable global), so it needs no live container.
//
// v1 caveat: a chosen name is recorded as the runner's intent (AtomicSelector)
// but the CE host sequence still presses the default-highlighted card
// (DefaultHostSequence) — walking the D-pad to a named card is follow-up work.
// The catalog is therefore what the player can ASK for, honestly surfaced as
// intent, until card-order navigation lands.

// CEMaps is the stock Halo: CE (Xbox) multiplayer map set.
var CEMaps = []string{
	"Battle Creek",
	"Sidewinder",
	"Damnation",
	"Rat Race",
	"Prisoner",
	"Hang 'Em High",
	"Chill Out",
	"Derelict",
	"Boarding Action",
	"Blood Gulch",
	"Wizard",
	"Longest",
	"Ice Fields",
}

// CEGametypes is the stock Halo: CE multiplayer game-type set.
var CEGametypes = []string{
	"Slayer",
	"Team Slayer",
	"Capture the Flag",
	"King of the Hill",
	"Oddball",
	"Race",
	"Juggernaut",
}

// Catalog is the map/gametype option set returned to the play tab.
type Catalog struct {
	Maps      []string `json:"maps"`
	Gametypes []string `json:"gametypes"`
}

// DefaultCatalog returns the CE display catalog.
func DefaultCatalog() Catalog {
	return Catalog{Maps: CEMaps, Gametypes: CEGametypes}
}
