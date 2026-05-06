package scraper

// PowerItemTracker tracks the live status of one power item spawn across ticks.
type PowerItemTracker struct {
	CurrentObjectID uint32 // handle & 0xFFFF; 0xFFFF = unknown
	Status          string // "world" | "held" | "respawning"
	HeldBy          int    // player index when held; -1 otherwise
	RespawnTimer    int32  // ticks remaining when respawning
}

// RosterEntry is one player's identity at a point in time. Used by event
// detection to diff joins, leaves, and team changes against the previous tick.
type RosterEntry struct {
	Name string
	Team uint32
}

// TickState holds per-instance inter-tick state for event detection.
// One TickState per xemu instance, owned exclusively by that instance's poll goroutine.
type TickState struct {
	// Static player data from previous tick (keyed by player index).
	PrevKills      map[int]int16
	PrevDeaths     map[int]int16
	PrevAssists    map[int]int16
	PrevTeamKills  map[int]int16
	PrevSuicides   map[int]int16
	PrevKillStreak map[int]uint16
	PrevMultikill  map[int]uint16
	PrevQuit       map[int]uint8

	// Dynamic player data from previous tick.
	PrevAlive          map[int]bool
	PrevHealth         map[int]float32
	PrevShields        map[int]float32
	PrevFrags          map[int]uint8
	PrevPlasmas        map[int]uint8
	PrevHasCamo        map[int]bool
	PrevHasOvershield  map[int]bool
	PrevParentObject   map[int]uint32 // vehicle handle (0xFFFFFFFF = on foot)
	PrevMeleeRemaining map[int]uint8

	// Previous weapon slots (objectID = handle & 0xFFFF; 0xFFFF = empty).
	PrevWeaponSlots map[int][4]uint32

	// Power item state (keyed by spawn_id).
	PowerItems map[int]*PowerItemTracker

	// Previous power item status for event detection.
	PrevPowerItemStatus map[int]string // spawn_id → "held"/"world"/"respawning"
	PrevPowerItemHeldBy map[int]int    // spawn_id → player index when held

	// Previous weapon ammo/charge for item_depleted detection.
	// Key: player*4+slot; value: ammo_mag or charge*1000 (int16)
	PrevWeaponAmmo   map[int]int16
	PrevWeaponEnergy map[int]float32

	// Previous roster (player index → identity) for player_joined /
	// player_left / player_team_changed event detection.
	PrevRoster map[int]RosterEntry

	// Previous per-team scores for team_score event detection.
	PrevTeamScores map[uint32]int32

	// Game state for game_start / game_end detection.
	PrevGameState GameState

	// Previous tick number.
	PrevTick uint32
}

// NewTickState initialises a TickState with all maps allocated.
func NewTickState() *TickState {
	return &TickState{
		PrevKills:           make(map[int]int16),
		PrevDeaths:          make(map[int]int16),
		PrevAssists:         make(map[int]int16),
		PrevTeamKills:       make(map[int]int16),
		PrevSuicides:        make(map[int]int16),
		PrevKillStreak:      make(map[int]uint16),
		PrevMultikill:       make(map[int]uint16),
		PrevQuit:            make(map[int]uint8),
		PrevAlive:           make(map[int]bool),
		PrevHealth:          make(map[int]float32),
		PrevShields:         make(map[int]float32),
		PrevFrags:           make(map[int]uint8),
		PrevPlasmas:         make(map[int]uint8),
		PrevHasCamo:         make(map[int]bool),
		PrevHasOvershield:   make(map[int]bool),
		PrevParentObject:    make(map[int]uint32),
		PrevMeleeRemaining:  make(map[int]uint8),
		PrevWeaponSlots:     make(map[int][4]uint32),
		PowerItems:          make(map[int]*PowerItemTracker),
		PrevPowerItemStatus: make(map[int]string),
		PrevPowerItemHeldBy: make(map[int]int),
		PrevWeaponAmmo:      make(map[int]int16),
		PrevWeaponEnergy:    make(map[int]float32),
		PrevRoster:          make(map[int]RosterEntry),
		PrevTeamScores:      make(map[uint32]int32),
	}
}

// InitPowerItems initialises power item trackers from the game data spawn list.
func (s *TickState) InitPowerItems(spawns []PowerItemSpawn) {
	s.PowerItems = make(map[int]*PowerItemTracker, len(spawns))
	s.PrevPowerItemStatus = make(map[int]string, len(spawns))
	s.PrevPowerItemHeldBy = make(map[int]int, len(spawns))
	for _, sp := range spawns {
		objectID := sp.InitialObjectID
		if objectID == 0 {
			objectID = 0xFFFF
		}
		s.PowerItems[sp.SpawnID] = &PowerItemTracker{
			CurrentObjectID: objectID,
			Status:          "world",
			HeldBy:          -1,
			RespawnTimer:    int32(sp.SpawnIntervalTicks),
		}
		s.PrevPowerItemStatus[sp.SpawnID] = "world"
		s.PrevPowerItemHeldBy[sp.SpawnID] = -1
	}
}
