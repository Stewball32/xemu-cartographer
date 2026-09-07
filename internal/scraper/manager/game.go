package manager

import "github.com/xemu-cartographer/xc-scraper/wire"

// The game class payload shapes are owned by the wire contract package
// (xc-scraper/wire/game.go); the names below are aliases so the builders in
// v2_adapters.go, the games persistence projection and every consumer keep
// one type identity. Field-level documentation (GameState vs Phase, the dead
// GameElapsedTicks, HostHealth, the acc_* accumulators) lives on the wire
// types.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`game`).
type (
	// GamePayload is the data for a game-class envelope. Config / TeamScores
	// / Players / Machines / Network are zero-valued (nil/empty) in the idle
	// phase; populated once the runner is in Ready or Live.
	GamePayload          = wire.GamePayload
	GameConfig           = wire.GameConfig
	GameTeamScore        = wire.GameTeamScore
	GameRosterPlayer     = wire.GameRosterPlayer
	GameMachine          = wire.GameMachine
	GameNetwork          = wire.GameNetwork
	GameNetworkCountdown = wire.GameNetworkCountdown
	GameNetworkClient    = wire.GameNetworkClient
)
