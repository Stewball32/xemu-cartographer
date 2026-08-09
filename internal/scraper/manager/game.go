package manager

import "time"

// envelopeTypeGame is the wire type for the per-instance game class —
// phase, freshness counters, config, roster, scores, machines, network.
// Change-driven with a ≤1 Hz heartbeat floor.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`game`).
const envelopeTypeGame = "game"

// GamePayload is the data for a game-class envelope. Config / TeamScores /
// Players / Machines / Network are zero-valued (nil/empty) in the idle
// phase; populated once the runner is in Ready or Live.
type GamePayload struct {
	Phase      Phase     `json:"phase"`
	StartedAt  time.Time `json:"started_at"`
	LastReadAt time.Time `json:"last_read_at"`
	EngineTick uint32    `json:"engine_tick"`
	// GameElapsedTicks is the MATCH-ELAPSED clock (game_time_globals+0x10), a 30Hz
	// count-UP from 0 at match start — what the scorebug renders as M:SS. EngineTick
	// above is the free-running frame counter and is NOT match-relative.
	GameElapsedTicks uint32 `json:"game_elapsed_ticks"`
	Iterations       uint64 `json:"iterations"`

	Config     *GameConfig        `json:"config"`
	TeamScores []GameTeamScore    `json:"team_scores"`
	Players    []GameRosterPlayer `json:"players"`
	Machines   []GameMachine      `json:"machines"`
	Network    *GameNetwork       `json:"network"`
}

type GameConfig struct {
	Gametype       string `json:"gametype"`
	VariantName    string `json:"variant_name,omitempty"`
	IsTeamGame     bool   `json:"is_team_game"`
	ScoreLimit     int32  `json:"score_limit"`
	TimeLimitTicks int32  `json:"time_limit_ticks"`
}

type GameTeamScore struct {
	Team  uint32 `json:"team"`
	Score int32  `json:"score"`
}

type GameRosterPlayer struct {
	Index           int    `json:"index"`
	Name            string `json:"name"`
	Team            uint32 `json:"team"`
	ArmorColor      int16  `json:"armor_color"`
	Score           int32  `json:"score"`
	Kills           int16  `json:"kills"`
	Deaths          int16  `json:"deaths"`
	Assists         int16  `json:"assists"`
	CTFScore        int16  `json:"ctf_score"`
	TeamKills       int16  `json:"team_kills"`
	Suicides        int16  `json:"suicides"`
	KillStreak      uint16 `json:"kill_streak"`
	Multikill       uint16 `json:"multikill"`
	ShotsFired      int32  `json:"shots_fired"`
	ShotsHit        int16  `json:"shots_hit"`
	IsLocal         *bool  `json:"is_local"`
	LocalIndex      *int   `json:"local_index"`
	MachineIndex    *int   `json:"machine_index"`
	ControllerIndex *int   `json:"controller_index"`
}

type GameMachine struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	IsLocal *bool  `json:"is_local"`
}

// GameNetwork moved here from the v1 TickPayload — it's slow-changing
// (ping, countdown, machine roster) and doesn't need 30 Hz cadence.
type GameNetwork struct {
	Countdown *GameNetworkCountdown `json:"countdown"`
	Client    *GameNetworkClient    `json:"client"`
}

type GameNetworkCountdown struct {
	Active         bool  `json:"active"`
	Paused         bool  `json:"paused"`
	SecondsToStart int16 `json:"seconds_to_start"`
}

type GameNetworkClient struct {
	MachineIndex uint16 `json:"machine_index"`
	AveragePing  int16  `json:"average_ping"`
	PacketsSent  int16  `json:"packets_sent"`
}
