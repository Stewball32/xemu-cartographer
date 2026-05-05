package haloce

import (
	"encoding/binary"
	"math"
	"unicode/utf16"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// Reader reads Halo: CE game state from a single xemu instance.
//
// All caches on this struct are scenario- or match-scoped. The lifecycle hook
// OnStateChange clears them on entry to GameStateMenu — the only path by
// which a different scenario can be loaded. See reader_cache.go.
type Reader struct {
	inst         *xemu.Instance
	name         string
	tagNameCache map[int16]string
	tagInstBase  uint32 // cached; 0 = not yet read
	ohdBase      uint32 // cached; 0 = not yet read

	// Static weapon-tag-data cache. Populated on first read for each tag index;
	// reused for the lifetime of the loaded scenario.
	weaponTagDataCache map[int16]*scraper.StaticWeaponTagData

	// Static biped-tag-data cache. Same lifetime as weaponTagDataCache.
	bipedTagCache map[int16]*scraper.StaticBipedTagData

	// Three-tier composition caches. Filled lazily inside ensureScenarioStatic
	// / ensureMatchStatic, dropped on entry to menu.
	scenarioCache *scenarioStaticCache
	matchCache    *matchStaticCache

	// lastStateInputs caches the raw values fed into determineGameState on the
	// most recent ReadGameState call. Surfaced via LastStateInputs for the
	// debug inspect endpoint.
	lastStateInputs scraper.StateInputs
}

// NewReader creates a Reader for the given instance.
// inst.Init(AllLowGVAs) must have been called before use.
func NewReader(inst *xemu.Instance, instanceName string) *Reader {
	return &Reader{
		inst:               inst,
		name:               instanceName,
		tagNameCache:       make(map[int16]string),
		weaponTagDataCache: make(map[int16]*scraper.StaticWeaponTagData),
		bipedTagCache:      make(map[int16]*scraper.StaticBipedTagData),
	}
}

// -------------------------------------------------------------------
// Game state (lightweight — called every iteration of the poll loop)
// -------------------------------------------------------------------

// ReadGameState reads the minimum needed to determine game state and current tick.
func (r *Reader) ReadGameState() (state scraper.GameState, tick uint32, err error) {
	inst := r.inst
	mem := inst.Mem

	geGlobalsPtr, err := inst.DerefLowPtr(AddrGameEngineGlobalsPtr)
	if err != nil {
		return scraper.GameStateMenu, 0, err
	}
	gameEngineRunning := geGlobalsPtr != 0

	mainMenuHVA, err := inst.LowHVA(AddrMainMenuActive)
	if err != nil {
		return scraper.GameStateMenu, 0, err
	}
	mainMenu, err := mem.ReadU8At(mainMenuHVA)
	if err != nil {
		return scraper.GameStateMenu, 0, err
	}

	gtgPtr, err := inst.DerefLowPtr(AddrGameTimeGlobalsPtr)
	if err != nil {
		return scraper.GameStateMenu, 0, err
	}

	var initialized, active, paused uint8
	if gtgPtr >= HighGVAThreshold {
		initialized, _ = mem.ReadU8(gtgPtr + OffGTGInitialized)
		active, _ = mem.ReadU8(gtgPtr + OffGTGActive)
		paused, _ = mem.ReadU8(gtgPtr + OffGTGPaused)
		tick, _ = mem.ReadU32(gtgPtr + OffGTGGameTime)
	}

	gameCanScoreHVA, err := inst.LowHVA(AddrGameCanScore)
	if err != nil {
		return scraper.GameStateMenu, tick, err
	}
	gameCanScore, _ := mem.ReadU32At(gameCanScoreHVA)

	state = determineGameState(mainMenu, initialized, active, paused, gameEngineRunning, gameCanScore)
	r.lastStateInputs = scraper.StateInputs{
		"main_menu":               mainMenu,
		"initialized":             initialized,
		"active":                  active,
		"paused":                  paused,
		"engine_running":          gameEngineRunning,
		"game_can_score":          gameCanScore,
		"game_engine_globals_ptr": geGlobalsPtr,
		"game_time_globals_ptr":   gtgPtr,
	}
	return state, tick, nil
}

// LastStateInputs returns the raw values from the most recent ReadGameState
// call. Returns nil before the first call.
func (r *Reader) LastStateInputs() scraper.StateInputs {
	return r.lastStateInputs
}

func determineGameState(mainMenu, initialized, active, paused uint8, engineRunning bool, gameCanScore uint32) scraper.GameState {
	if mainMenu != 0 || initialized == 0 {
		return scraper.GameStateMenu
	}
	if initialized == 1 && active == 0 && paused == 1 {
		return scraper.GameStatePreGame
	}
	if initialized == 1 && active == 1 && paused == 0 {
		if engineRunning && gameCanScore != 0 {
			return scraper.GameStatePostGame
		}
		return scraper.GameStateInGame
	}
	return scraper.GameStateMenu
}

// -------------------------------------------------------------------
// Snapshot (called on game-state transition / client connect)
// -------------------------------------------------------------------

// ReadGameData returns the "current state" message — composed from the
// scenario- and match-static caches plus a fresh read of live volatile
// fields (roster, scores, match config). After warm caches this is a
// cheap call (~50–100 memory reads) suitable for per-iteration refresh.
func (r *Reader) ReadGameData() (scraper.GameData, error) {
	r.ensureScenarioStatic()
	r.ensureMatchStatic()
	return r.composeGameData(), nil
}

// ReadReadyState is the cheap variant of ReadGameData intended for the
// manager loop's per-iteration refresh in the Ready phase. Identical
// implementation — the name distinguishes the call site so loop code reads
// as "refresh the ready-phase view" rather than "rebuild the full game
// data."
func (r *Reader) ReadReadyState() (scraper.GameData, error) {
	return r.ReadGameData()
}

// composeGameData merges cached scenario- and match-static data with live
// reads of volatile fields (match config, roster, team scores, power-item
// world status) into a GameData. The wire shape matches the legacy
// output exactly — frontend types in sveltekit/src/lib/types/scraper.ts
// continue to work unchanged.
func (r *Reader) composeGameData() scraper.GameData {
	out := scraper.GameData{}

	// Live match-config fields. Cheap; the host can still change these in
	// pregame, so read every call rather than caching.
	isTeamGameHVA, _ := r.inst.LowHVA(AddrIsTeamGame)
	isTeamGameV, _ := r.inst.Mem.ReadU8At(isTeamGameHVA)
	out.IsTeamGame = isTeamGameV != 0

	gametypeID, err := r.readGametypeID()
	if err != nil {
		gametypeID = 0
	}
	if name := GametypeNames[gametypeID]; name != "" {
		out.Gametype = name
	} else {
		out.Gametype = "unknown"
	}
	out.VariantName = r.readVariantName()
	out.ScoreLimit, _ = r.readScoreLimit(gametypeID)
	out.TeamScores, _ = r.readTeamScores(out.IsTeamGame, gametypeID)
	out.Players, _ = r.readGamePlayers()
	if len(out.Players) == 0 {
		// PlayerDatumArray is empty in lobby states (splitscreen / system-link
		// pre-match). Fall back to the network-game-data roster so the debug
		// page sees lobby joins immediately.
		out.Players, _ = r.readNetworkRosterPlayers()
	}
	out.Machines = r.readNetworkMachines()
	r.attributeMachines(out.Players)
	r.fillPlayerScores(out.Players, gametypeID)
	out.TimeLimitTicks = 0 // no verified address

	// Scenario-static data — fall through to live reads if cache hasn't filled
	// yet (e.g. very early pregame when the scenario pointer is still null).
	if r.scenarioCache != nil && r.scenarioCache.Filled {
		out.Map = r.scenarioCache.MapName
		out.GameDifficulty = r.scenarioCache.GameDifficulty
		out.PlayerSpawns = r.scenarioCache.PlayerSpawns
		out.Fog = r.scenarioCache.Fog
		out.ObjectTypes = r.scenarioCache.ObjectTypes
		out.TagCache = r.scenarioCache.TagCache
		out.PowerItemSpawns = r.composePowerItemSpawns()
	} else {
		out.Map = r.readLowString(AddrMultiplayerMapName, 32)
	}

	return out
}

// composePowerItemSpawns rebuilds the legacy []scraper.PowerItemSpawn slice
// by joining scenario-static positions/tags/intervals with match-static
// initial object IDs. When the OIDs cache hasn't filled yet, InitialObjectID
// stays 0xFFFF (the existing sentinel for "not found").
func (r *Reader) composePowerItemSpawns() []scraper.PowerItemSpawn {
	if r.scenarioCache == nil || len(r.scenarioCache.PowerSpawnsScenario) == 0 {
		return nil
	}
	scen := r.scenarioCache.PowerSpawnsScenario
	out := make([]scraper.PowerItemSpawn, 0, len(scen))
	for _, sp := range scen {
		oid := uint32(0xFFFF)
		if r.matchCache != nil && r.matchCache.InitialObjIDsFilled {
			if v, ok := r.matchCache.PowerInitialOIDs[sp.SpawnID]; ok {
				oid = v
			}
		}
		out = append(out, scraper.PowerItemSpawn{
			SpawnID:            sp.SpawnID,
			Tag:                sp.Tag,
			SpawnIntervalTicks: sp.SpawnIntervalTicks,
			X:                  sp.X,
			Y:                  sp.Y,
			Z:                  sp.Z,
			InitialObjectID:    oid,
		})
	}
	return out
}

// readVariantName returns the host's loaded variant name (e.g. "TS TRAINING",
// "CTF 3C 10S", "Accumulate"). Stored as UTF-16-LE in the first 24 bytes
// (12 chars max) of the variant struct at RefAddrGlobalVariant. Updated
// at match-start, so in lobby this is the *last loaded* variant rather
// than the dropdown selection.
func (r *Reader) readVariantName() string {
	hva, err := r.inst.LowHVA(RefAddrGlobalVariant)
	if err != nil {
		return ""
	}
	b, err := r.inst.Mem.ReadBytesAt(hva, 24)
	if err != nil {
		return ""
	}
	return decodeUTF16LE(b)
}

// readGametypeID returns the current gametype ID (1=ctf, 2=slayer, 3=oddball,
// 4=king, 5=race, 6=terminator, 7=stub) by reading the u32 at
// RefAddrGlobalVariant + OffGVGametype. The variant struct holds the
// running variant once a match has started; engine-globals presence gates
// the read so we don't surface stale "last loaded" data during the lobby.
//
// Confirmed via probe: in active CTF/Slayer/Oddball matches the value at
// +0x18 of this struct matches the running gametype. Both bases (0x2F90A8
// and 0x2FAB60) carry identical bytes mid-match; we use the first.
func (r *Reader) readGametypeID() (uint32, error) {
	geGlobalsPtr, err := r.inst.DerefLowPtr(AddrGameEngineGlobalsPtr)
	if err != nil {
		return 0, err
	}
	if geGlobalsPtr == 0 {
		return 0, nil
	}
	hva, err := r.inst.LowHVA(RefAddrGlobalVariant)
	if err != nil {
		return 0, err
	}
	return r.inst.Mem.ReadU32At(hva + int64(OffGVGametype))
}

// fillPlayerScores populates each player's Score field using the per-gametype
// score table. CTF reuses the static-player ctf_score s16 (already read into
// CTFScore by readGamePlayer). Slayer/Oddball/King/Race read s32 from the
// per-player slot at score_base + PlayerScoreBaseOffset + 4*idx — all four
// tables live in their own memory bases (see offsets.go AddrScore*). Unknown
// gametypes leave Score=0.
func (r *Reader) fillPlayerScores(players []scraper.GamePlayer, gametypeID uint32) {
	if len(players) == 0 {
		return
	}
	if gametypeID == 1 {
		// CTF — per-player score lives on the static-player struct.
		for i := range players {
			players[i].Score = int32(players[i].CTFScore)
		}
		return
	}
	var baseAddr uint32
	switch gametypeID {
	case 2:
		baseAddr = AddrScoreSlayer
	case 3:
		baseAddr = AddrScoreOddball
	case 4:
		baseAddr = AddrScoreKing
	case 5:
		baseAddr = AddrScoreRace
	default:
		return
	}
	hva, err := r.inst.LowHVA(baseAddr)
	if err != nil {
		return
	}
	tableHVA := hva + int64(PlayerScoreBaseOffset)
	for i := range players {
		v, err := r.inst.Mem.ReadU32At(tableHVA + int64(players[i].Index)*4)
		if err != nil {
			continue
		}
		players[i].Score = int32(v)
	}
}

// readScoreLimit returns the active score limit. Prefers the limit matching
// the supplied gametypeID; falls back to the first non-zero limit when the
// gametype-specific value is zero or gametypeID is unknown (e.g. pregame
// before the engine globals pointer initialises).
func (r *Reader) readScoreLimit(gametypeID uint32) (int32, error) {
	type entry struct {
		gametype uint32
		addr     uint32
	}
	addrs := []entry{
		{1, AddrScoreLimitCTF},
		{2, AddrScoreLimitSlayer},
		{3, AddrScoreLimitOddball},
	}
	values := make(map[uint32]int32, len(addrs))
	for _, a := range addrs {
		hva, err := r.inst.LowHVA(a.addr)
		if err != nil {
			continue
		}
		v, err := r.inst.Mem.ReadU32At(hva)
		if err != nil {
			continue
		}
		values[a.gametype] = int32(v)
	}
	if v, ok := values[gametypeID]; ok && v != 0 {
		return v, nil
	}
	for _, a := range addrs {
		if v := values[a.gametype]; v != 0 {
			return v, nil
		}
	}
	return 0, nil
}

// readTeamScores returns the per-team scores (red, blue) for team games. The
// score base differs per gametype — see AddrScore* in offsets.go and the
// matching halocaster.py team_score_addresses_by_gametype map.
func (r *Reader) readTeamScores(isTeamGame bool, gametypeID uint32) ([]scraper.TeamScore, error) {
	if !isTeamGame {
		return nil, nil
	}
	var baseAddr uint32
	switch gametypeID {
	case 1:
		baseAddr = AddrScoreCTF
	case 2:
		baseAddr = AddrScoreSlayer
	case 3:
		baseAddr = AddrScoreOddball
	case 4:
		baseAddr = AddrScoreKing
	case 5:
		baseAddr = AddrScoreRace
	default:
		return nil, nil
	}
	hva, err := r.inst.LowHVA(baseAddr)
	if err != nil {
		return nil, err
	}
	red, err := r.inst.Mem.ReadU32At(hva)
	if err != nil {
		return nil, err
	}
	blue, err := r.inst.Mem.ReadU32At(hva + 4)
	if err != nil {
		return nil, err
	}
	return []scraper.TeamScore{
		{Team: 0, Score: int32(red)},
		{Team: 1, Score: int32(blue)},
	}, nil
}

// readNetworkRosterPlayers reads the lobby roster from network_game_data's
// network_players table — the source of truth for players who have joined
// the lobby but whose in-engine PlayerDatum slots aren't allocated yet
// (system-link / splitscreen pre-match). Each entry is a 32-byte record:
// wchar name (24 bytes), s16 color, s16 unused, u8 machine, u8 controller,
// u8 team, u8 player_list_index. Atlas: halocaster.py:1228-1235.
//
// Returns a sparse GamePlayer with Index/Name/Team/MachineIndex populated.
// IsLocal/LocalIndex are set only when the player's machine_index matches
// this xemu instance's own machine_index (read from network_game_client) —
// the controller_index field is the controller slot on the player's *own*
// machine, not a "is this player local to me" signal.
// Kill/death/score fields stay zero — they don't exist pre-match.
func (r *Reader) readNetworkRosterPlayers() ([]scraper.GamePlayer, error) {
	mem := r.inst.Mem
	clientHVA, err := r.inst.LowHVA(RefAddrNetworkGameClient)
	if err != nil {
		return nil, err
	}
	ngdHVA := clientHVA + int64(OffNGCNetworkGameData)
	playerCount, err := mem.ReadS16At(ngdHVA + int64(OffNGDPlayerCount))
	if err != nil || playerCount <= 0 {
		return nil, err
	}
	ownMachine, _ := mem.ReadU16At(clientHVA + int64(OffNGCMachineIndex))
	rosterHVA := ngdHVA + int64(OffNGDNetworkPlayers)
	players := make([]scraper.GamePlayer, 0, playerCount)
	for i := int16(0); i < playerCount; i++ {
		entryHVA := rosterHVA + int64(uint32(i)*NetworkPlayerStride)
		nameBytes, err := mem.ReadBytesAt(entryHVA+int64(OffNetPlayerName), 24)
		if err != nil || (nameBytes[0] == 0 && nameBytes[1] == 0) {
			continue
		}
		team, _ := mem.ReadU8At(entryHVA + int64(OffNetPlayerTeam))
		ctrl, _ := mem.ReadU8At(entryHVA + int64(OffNetPlayerControllerIndex))
		machine, _ := mem.ReadU8At(entryHVA + int64(OffNetPlayerMachineIndex))
		listIdx, _ := mem.ReadU8At(entryHVA + int64(OffNetPlayerListIndex))

		isLocal := uint16(machine) == ownMachine
		var localIdx *int
		if isLocal {
			v := int(ctrl)
			localIdx = &v
		}
		machineIdx := int(machine)
		players = append(players, scraper.GamePlayer{
			Index:        int(listIdx),
			Name:         decodeUTF16LE(nameBytes),
			Team:         uint32(team),
			IsLocal:      &isLocal,
			LocalIndex:   localIdx,
			MachineIndex: &machineIdx,
		})
	}
	return players, nil
}

// readNetworkMachines reads the connected-machine roster from
// network_game_data.network_machines (atlas:1224-1226). Each entry is a
// 68-byte record: wchar name (64 bytes / 32 chars) followed by a u8
// machine_index. Returns nil when no network game is active.
func (r *Reader) readNetworkMachines() []scraper.GameMachine {
	mem := r.inst.Mem
	clientHVA, err := r.inst.LowHVA(RefAddrNetworkGameClient)
	if err != nil {
		return nil
	}
	ngdHVA := clientHVA + int64(OffNGCNetworkGameData)
	machineCount, err := mem.ReadS16At(ngdHVA + int64(OffNGDMachineCount))
	if err != nil || machineCount <= 0 {
		return nil
	}
	rosterHVA := ngdHVA + int64(OffNGDNetworkMachines)
	machines := make([]scraper.GameMachine, 0, machineCount)
	for i := int16(0); i < machineCount; i++ {
		entryHVA := rosterHVA + int64(uint32(i)*NetworkMachineStride)
		nameBytes, err := mem.ReadBytesAt(entryHVA+int64(OffNetMachineName), 64)
		if err != nil {
			continue
		}
		idx, _ := mem.ReadU8At(entryHVA + int64(OffNetMachineMachineIndex))
		name := decodeUTF16LE(nameBytes)
		if name == "" {
			continue
		}
		machines = append(machines, scraper.GameMachine{
			Index: int(idx),
			Name:  name,
		})
	}
	return machines
}

// attributeMachines fills GamePlayer.MachineIndex by joining each player
// against the network_players roster by name. Necessary because the in-engine
// PlayerDatumArray (the source of in-game roster reads) doesn't carry a
// machine index — that field only exists in the network roster, which is
// also what the lobby debug page needs to show "who connected from where"
// regardless of whether PDA is populated yet.
func (r *Reader) attributeMachines(players []scraper.GamePlayer) {
	if len(players) == 0 {
		return
	}
	mem := r.inst.Mem
	clientHVA, err := r.inst.LowHVA(RefAddrNetworkGameClient)
	if err != nil {
		return
	}
	ngdHVA := clientHVA + int64(OffNGCNetworkGameData)
	playerCount, err := mem.ReadS16At(ngdHVA + int64(OffNGDPlayerCount))
	if err != nil || playerCount <= 0 {
		return
	}
	rosterHVA := ngdHVA + int64(OffNGDNetworkPlayers)
	nameToMachine := make(map[string]int, playerCount)
	for i := int16(0); i < playerCount; i++ {
		entryHVA := rosterHVA + int64(uint32(i)*NetworkPlayerStride)
		nameBytes, err := mem.ReadBytesAt(entryHVA+int64(OffNetPlayerName), 24)
		if err != nil {
			continue
		}
		machine, _ := mem.ReadU8At(entryHVA + int64(OffNetPlayerMachineIndex))
		nameToMachine[decodeUTF16LE(nameBytes)] = int(machine)
	}
	for i := range players {
		if players[i].MachineIndex != nil {
			continue
		}
		if mi, ok := nameToMachine[players[i].Name]; ok {
			v := mi
			players[i].MachineIndex = &v
		}
	}
}

func (r *Reader) readGamePlayers() ([]scraper.GamePlayer, error) {
	inst := r.inst
	mem := inst.Mem

	pdaBase, err := inst.DerefLowPtr(AddrPlayerDatumArrayPtr)
	if err != nil || pdaBase < HighGVAThreshold {
		return nil, err
	}
	elemSize, _ := mem.ReadU16(pdaBase + OffPDAElementSize)
	currentCount, _ := mem.ReadU16(pdaBase + OffPDACurrentCount)
	firstElement, _ := mem.ReadU32(pdaBase + OffPDAFirstElement)
	if firstElement < HighGVAThreshold || currentCount == 0 || elemSize == 0 {
		return nil, nil
	}

	players := make([]scraper.GamePlayer, 0, currentCount)
	for i := uint16(0); i < currentCount; i++ {
		base := firstElement + uint32(i)*uint32(elemSize)
		p, ok, err := r.readGamePlayer(int(i), base)
		if err != nil || !ok {
			continue
		}
		players = append(players, p)
	}
	return players, nil
}

func (r *Reader) readGamePlayer(index int, base uint32) (scraper.GamePlayer, bool, error) {
	mem := r.inst.Mem

	nameBytes, err := mem.ReadBytes(base+OffPlrName, 24)
	if err != nil {
		return scraper.GamePlayer{}, false, err
	}
	if nameBytes[0] == 0 && nameBytes[1] == 0 {
		return scraper.GamePlayer{}, false, nil
	}

	team, _ := mem.ReadU32(base + OffPlrTeam)
	kills, _ := mem.ReadS16(base + OffPlrKills)
	deaths, _ := mem.ReadS16(base + OffPlrDeaths)
	assists, _ := mem.ReadS16(base + OffPlrAssists)
	teamKills, _ := mem.ReadS16(base + OffPlrTeamKills)
	suicides, _ := mem.ReadS16(base + OffPlrSuicides)
	ctfScore, _ := mem.ReadS16(base + OffPlrCTFScore)
	ksRaw, _ := mem.ReadU16(base + OffPlrKillStreak)
	mkRaw, _ := mem.ReadU16(base + OffPlrMultikill)
	sfRaw, _ := mem.ReadS32(base + OffPlrShotsFired)
	shRaw, _ := mem.ReadS16(base + OffPlrShotsHit)
	li, _ := mem.ReadS16(base + OffPlrLocalIndex)

	isLocal := li >= 0
	var localIdx *int
	if isLocal {
		v := int(li)
		localIdx = &v
	}

	return scraper.GamePlayer{
		Index:      index,
		Name:       decodeUTF16LE(nameBytes),
		Team:       team,
		Kills:      kills,
		Deaths:     deaths,
		Assists:    assists,
		CTFScore:   ctfScore,
		TeamKills:  teamKills,
		Suicides:   suicides,
		KillStreak: ksRaw,
		Multikill:  mkRaw,
		ShotsFired: sfRaw,
		ShotsHit:   shRaw,
		IsLocal:    &isLocal,
		LocalIndex: localIdx,
	}, true, nil
}

// -------------------------------------------------------------------
// Tick (called every game tick when in_game)
// -------------------------------------------------------------------

// ReadTick reads all 30Hz dynamic state for one tick.
func (r *Reader) ReadTick(spawns []scraper.PowerItemSpawn, state *scraper.TickState) (scraper.TickResult, error) {
	inst := r.inst
	mem := inst.Mem

	// Ensure cached bases are populated.
	if err := r.ensureBases(); err != nil {
		return scraper.TickResult{}, err
	}

	// Match-static cache feeds readLocals (UI globals, look rates) and the
	// power-item InitialObjectIDs. Idempotent + cheap when warm; the call
	// also runs the mid-pregame splitscreen-join refresh.
	r.ensureMatchStatic()

	// Re-read object header first_element_address every tick (table rearranges every ~30s).
	var objHeaderFirst uint32
	var objElemSize uint16
	var objAllocCount uint16
	if r.ohdBase >= HighGVAThreshold {
		objElemSize, _ = mem.ReadU16(r.ohdBase + OffOHDElementSize)
		objHeaderFirst, _ = mem.ReadU32(r.ohdBase + OffOHDFirstElement)
		objAllocCount, _ = mem.ReadU16(r.ohdBase + OffOHDAllocCount)
	}

	// Read player datum array.
	pdaBase, err := inst.DerefLowPtr(AddrPlayerDatumArrayPtr)
	if err != nil || pdaBase < HighGVAThreshold {
		return scraper.TickResult{}, err
	}
	elemSize, _ := mem.ReadU16(pdaBase + OffPDAElementSize)
	currentCount, _ := mem.ReadU16(pdaBase + OffPDACurrentCount)
	firstElement, _ := mem.ReadU32(pdaBase + OffPDAFirstElement)
	if firstElement < HighGVAThreshold || elemSize == 0 {
		return scraper.TickResult{}, nil
	}

	tickPlayers := make([]scraper.TickPlayer, 0, currentCount)
	internalPlayers := make([]scraper.InternalPlayerState, 0, currentCount)

	for i := uint16(0); i < currentCount; i++ {
		playerBase := firstElement + uint32(i)*uint32(elemSize)
		tp, ip, ok, err := r.readTickPlayer(int(i), playerBase, objHeaderFirst, objElemSize)
		if err != nil || !ok {
			continue
		}
		tickPlayers = append(tickPlayers, tp)
		internalPlayers = append(internalPlayers, ip)
	}

	// Build playerSlots map: objectID → player index (for power item held detection).
	playerSlots := make(map[uint32]int, len(internalPlayers)*4)
	for _, ip := range internalPlayers {
		for _, handle := range ip.WeaponSlots {
			if handle != HandleEmpty {
				oid := handle & HandleIndexMask
				playerSlots[oid] = ip.Index
			}
		}
	}

	powerItems := r.readPowerItemStatus(spawns, state, playerSlots, objHeaderFirst, objElemSize, objAllocCount)

	localCount := r.readLocalPlayerCount()
	result := scraper.TickResult{
		Payload: scraper.TickPayload{
			Players:     tickPlayers,
			PowerItems:  powerItems,
			GameGlobals: r.readGameGlobals(),
			PlayerCount: int16(currentCount),
			LocalCount:  localCount,
			Locals:      r.readLocals(localCount),
			Objects:     r.readObjects(),
			Network:     r.readNetwork(),
			DataQueue:   r.readDataQueue(),
			CTFFlags:    r.readCTFFlags(),
			Projectiles: r.readProjectiles(),
		},
		InternalPlayers: internalPlayers,
	}
	return result, nil
}

func (r *Reader) readTickPlayer(
	index int,
	playerBase uint32,
	objHeaderFirst uint32,
	objElemSize uint16,
) (scraper.TickPlayer, scraper.InternalPlayerState, bool, error) {
	mem := r.inst.Mem

	// Active slot check: name's first UTF-16 char must be non-zero. Read the
	// full 24-byte name buffer up-front so it can also be decoded into the
	// broadcast roster field below.
	nameBytes, err := mem.ReadBytes(playerBase+OffPlrName, 24)
	if err != nil {
		return scraper.TickPlayer{}, scraper.InternalPlayerState{}, false, err
	}
	if nameBytes[0] == 0 && nameBytes[1] == 0 {
		return scraper.TickPlayer{}, scraper.InternalPlayerState{}, false, nil
	}

	// Static fields.
	ip := scraper.InternalPlayerState{Index: index}
	ip.Kills, _ = mem.ReadS16(playerBase + OffPlrKills)
	ip.Deaths, _ = mem.ReadS16(playerBase + OffPlrDeaths)
	ip.Assists, _ = mem.ReadS16(playerBase + OffPlrAssists)
	ip.TeamKills, _ = mem.ReadS16(playerBase + OffPlrTeamKills)
	ip.Suicides, _ = mem.ReadS16(playerBase + OffPlrSuicides)
	ksRaw, _ := mem.ReadU16(playerBase + OffPlrKillStreak)
	mkRaw, _ := mem.ReadU16(playerBase + OffPlrMultikill)
	ip.KillStreak = ksRaw
	ip.Multikill = mkRaw
	ip.ShotsFired, _ = mem.ReadS32(playerBase + OffPlrShotsFired)
	ip.ShotsHit, _ = mem.ReadS16(playerBase + OffPlrShotsHit)
	quitRaw, _ := mem.ReadU8(playerBase + OffPlrQuit)
	ip.QuitFlag = quitRaw
	respawnRaw, _ := mem.ReadU32(playerBase + OffPlrRespawnTimer)
	ip.RespawnTimer = respawnRaw

	handle, _ := mem.ReadS32(playerBase + OffPlrObjectHandle)
	prevHandle, _ := mem.ReadS32(playerBase + OffPlrPrevObjHandle)

	alive := handle != -1

	tp := scraper.TickPlayer{
		Index: index,
		Alive: alive,
	}

	if !alive && respawnRaw > 0 {
		v := respawnRaw
		tp.RespawnInTicks = &v
	}

	// Dynamic object data.
	if alive && objHeaderFirst >= HighGVAThreshold && objElemSize > 0 {
		objIdx := uint32(handle) & HandleIndexMask
		objEntryAddr := objHeaderFirst + objIdx*uint32(objElemSize)
		objDataAddr, _ := mem.ReadU32(objEntryAddr + OffObjEntryDataAddr)
		if objDataAddr >= HighGVAThreshold {
			ip.ObjDataAddr = objDataAddr
			r.readDynPlayerFull(&tp, &ip, objDataAddr)
			tp.Extended = r.readDynPlayerExtended(objDataAddr)
			tp.Bones = r.readBones(objDataAddr)
			bipedTagIdx, _ := mem.ReadS16(objDataAddr + OffObjTagIndex)
			tp.BipedTag = r.readBipedTagData(bipedTagIdx)
		}
	}

	// Update-queue slot (input replication state). Available for both alive
	// and dead players, and for both locals and remotes.
	tp.UpdateQueue = r.readPlayerUpdateQueue(index)

	// Previous biped for damage table reads when dead.
	if !alive && prevHandle != -1 && objHeaderFirst >= HighGVAThreshold && objElemSize > 0 {
		prevIdx := uint32(prevHandle) & HandleIndexMask
		prevEntry := objHeaderFirst + prevIdx*uint32(objElemSize)
		prevAddr, _ := mem.ReadU32(prevEntry + OffObjEntryDataAddr)
		if prevAddr >= HighGVAThreshold {
			ip.PrevObjDataAddr = prevAddr
			ip.DamageTable = r.readDamageTable(prevAddr)
		}
	}
	if alive && ip.ObjDataAddr >= HighGVAThreshold {
		ip.DamageTable = r.readDamageTable(ip.ObjDataAddr)
	}

	return tp, ip, true, nil
}

func (r *Reader) readDynPlayerFull(tp *scraper.TickPlayer, ip *scraper.InternalPlayerState, objDataAddr uint32) {
	mem := r.inst.Mem

	tp.X, _ = mem.ReadF32(objDataAddr + OffDynX)
	tp.Y, _ = mem.ReadF32(objDataAddr + OffDynY)
	tp.Z, _ = mem.ReadF32(objDataAddr + OffDynZ)
	tp.VX, _ = mem.ReadF32(objDataAddr + OffDynVelX)
	tp.VY, _ = mem.ReadF32(objDataAddr + OffDynVelY)
	tp.VZ, _ = mem.ReadF32(objDataAddr + OffDynVelZ)
	tp.AimX, _ = mem.ReadF32(objDataAddr + OffDynAimX)
	tp.AimY, _ = mem.ReadF32(objDataAddr + OffDynAimY)
	tp.AimZ, _ = mem.ReadF32(objDataAddr + OffDynAimZ)

	zoomRaw, _ := mem.ReadU8(objDataAddr + OffDynZoomLevel)
	tp.ZoomLevel = int8(zoomRaw)
	tp.CrouchScale, _ = mem.ReadF32(objDataAddr + OffDynCrouchScale)
	tp.Health, _ = mem.ReadF32(objDataAddr + OffDynHealth)
	tp.Shields, _ = mem.ReadF32(objDataAddr + OffDynShields)

	shieldsStatus, _ := mem.ReadU16(objDataAddr + OffDynShieldsStatus)
	tp.HasOvershield = (shieldsStatus & ShieldsStatusOvershield) != 0

	camo, _ := mem.ReadU8(objDataAddr + OffDynCamo)
	tp.HasCamo = camo == CamoStateActive

	fragsRaw, _ := mem.ReadU8(objDataAddr + OffDynFrags)
	plasmasRaw, _ := mem.ReadU8(objDataAddr + OffDynPlasmas)
	tp.Frags = fragsRaw
	tp.Plasmas = plasmasRaw

	selectedSlot, _ := mem.ReadS16(objDataAddr + OffDynSelectedSlot)
	tp.SelectedWeaponSlot = selectedSlot

	action, _ := mem.ReadU32(objDataAddr + OffDynCurrentAction)
	tp.IsCrouching = action&ActionCrouch != 0
	tp.IsJumping = action&ActionJump != 0
	tp.IsFiring = action&ActionFire != 0
	tp.IsShooting = action&ActionShooting != 0
	tp.IsFlashlightOn = action&ActionFlashlight != 0
	tp.IsThrowingGrenade = action&ActionGrenade != 0
	tp.IsPressingAction = action&ActionPressAction != 0
	tp.IsHoldingAction = action&ActionHoldAction != 0

	meleeRem, _ := mem.ReadU8(objDataAddr + OffDynMeleeRemaining)
	meleeDmg, _ := mem.ReadU8(objDataAddr + OffDynMeleeDamageTick)
	ip.MeleeRemaining = meleeRem
	ip.MeleeDamageTick = meleeDmg
	tp.IsMeleeing = meleeRem > 0

	parentObj, _ := mem.ReadU32(objDataAddr + OffDynParentObject)
	ip.ParentObject = parentObj

	// Weapon slots.
	wepOffsets := [4]uint32{OffDynWeaponSlot0, OffDynWeaponSlot1, OffDynWeaponSlot2, OffDynWeaponSlot3}
	for slot, off := range wepOffsets {
		handle, _ := mem.ReadU32(objDataAddr + off)
		ip.WeaponSlots[slot] = handle
		if handle != HandleEmpty {
			wi, ok, _ := r.readWeaponInfo(slot, handle)
			if ok {
				tp.Weapons = append(tp.Weapons, wi)
			}
		}
	}
}

func (r *Reader) readWeaponInfo(slot int, handle uint32) (scraper.WeaponInfo, bool, error) {
	if handle == HandleEmpty || r.ohdBase < HighGVAThreshold {
		return scraper.WeaponInfo{}, false, nil
	}
	mem := r.inst.Mem

	objElemSize, _ := mem.ReadU16(r.ohdBase + OffOHDElementSize)
	objHeaderFirst, _ := mem.ReadU32(r.ohdBase + OffOHDFirstElement)
	if objHeaderFirst < HighGVAThreshold || objElemSize == 0 {
		return scraper.WeaponInfo{}, false, nil
	}

	objIdx := handle & HandleIndexMask
	entryAddr := objHeaderFirst + objIdx*uint32(objElemSize)
	objDataAddr, _ := mem.ReadU32(entryAddr + OffObjEntryDataAddr)
	if objDataAddr < HighGVAThreshold {
		return scraper.WeaponInfo{}, false, nil
	}

	flags, _ := mem.ReadU32(objDataAddr + OffObjFlags)
	if flags&ObjFlagGarbage != 0 {
		return scraper.WeaponInfo{}, false, nil
	}

	tagIdx, _ := mem.ReadS16(objDataAddr + OffObjTagIndex)
	tagName, _ := r.readTagName(tagIdx)

	isEnergy := false
	if r.tagInstBase >= HighGVAThreshold {
		tagInstEntry := r.tagInstBase + uint32(TagInstStride)*uint32(uint16(tagIdx))
		tagDataPtr, _ := mem.ReadU32(tagInstEntry + OffTagDataPtr)
		if tagDataPtr >= HighGVAThreshold {
			weaponTypeRaw, _ := mem.ReadU8(tagDataPtr + OffWepTagWeaponType)
			isEnergy = weaponTypeRaw&EnergyWeaponMask != 0
		}
	}

	wi := scraper.WeaponInfo{
		Slot:     slot,
		ObjectID: handle & HandleIndexMask,
		Tag:      tagName,
		IsEnergy: isEnergy,
	}

	if isEnergy {
		charge, _ := mem.ReadF32(objDataAddr + OffWepCharge)
		wi.Charge = &charge
	} else {
		mag, _ := mem.ReadS16(objDataAddr + OffWepAmmoMag)
		pack, _ := mem.ReadS16(objDataAddr + OffWepAmmoPack)
		wi.AmmoMag = &mag
		wi.AmmoPack = &pack
	}

	wi.Extended = r.readWeaponObjectExtended(objDataAddr)
	wi.TagData = r.readWeaponTagData(tagIdx)

	return wi, true, nil
}

func (r *Reader) readDamageTable(objDataAddr uint32) [scraper.DamageTableSlots]scraper.DamageEntry {
	mem := r.inst.Mem
	var table [scraper.DamageTableSlots]scraper.DamageEntry
	base := objDataAddr + OffDynDamageTable
	for i := 0; i < scraper.DamageTableSlots; i++ {
		off := base + uint32(i)*DamageEntrySize
		t, _ := mem.ReadU32(off + OffDmgTime)
		a, _ := mem.ReadF32(off + OffDmgAmount)
		doh, _ := mem.ReadU32(off + OffDmgDealerObjHdl)
		dph, _ := mem.ReadU32(off + OffDmgDealerPlrHdl)
		table[i] = scraper.DamageEntry{DamageTime: t, Amount: a, DealerObjHandle: doh, DealerPlrHandle: dph}
	}
	return table
}

// -------------------------------------------------------------------
// Power item status
// -------------------------------------------------------------------

func (r *Reader) readPowerItemStatus(
	spawns []scraper.PowerItemSpawn,
	state *scraper.TickState,
	playerSlots map[uint32]int, // objectID → player index
	objHeaderFirst uint32,
	objElemSize uint16,
	objAllocCount uint16,
) []scraper.PowerItemStatus {
	if len(spawns) == 0 {
		return nil
	}

	mem := r.inst.Mem

	// Build world object map: tag → list of (objectID, x, y, z).
	type worldObj struct {
		objectID uint32
		x, y, z  float32
	}
	worldByTag := make(map[string][]worldObj)

	if objHeaderFirst >= HighGVAThreshold && objElemSize > 0 {
		for i := uint16(0); i < objAllocCount; i++ {
			entryAddr := objHeaderFirst + uint32(i)*uint32(objElemSize)
			objDataAddr, _ := mem.ReadU32(entryAddr + OffObjEntryDataAddr)
			if objDataAddr < HighGVAThreshold {
				continue
			}
			flags, _ := mem.ReadU32(objDataAddr + OffObjFlags)
			if flags&ObjFlagGarbage != 0 {
				continue
			}
			tagIdx, _ := mem.ReadS16(objDataAddr + OffObjTagIndex)
			tagName, err := r.readTagName(tagIdx)
			if err != nil || tagName == "" {
				continue
			}
			x, _ := mem.ReadF32(objDataAddr + OffObjX)
			y, _ := mem.ReadF32(objDataAddr + OffObjY)
			z, _ := mem.ReadF32(objDataAddr + OffObjZ)
			worldByTag[tagName] = append(worldByTag[tagName], worldObj{uint32(i), x, y, z})
		}
	}

	result := make([]scraper.PowerItemStatus, len(spawns))
	for idx, spawn := range spawns {
		tracker, ok := state.PowerItems[spawn.SpawnID]
		if !ok {
			tracker = &scraper.PowerItemTracker{CurrentObjectID: 0xFFFF, Status: "respawning", HeldBy: -1}
			state.PowerItems[spawn.SpawnID] = tracker
		}

		status := scraper.PowerItemStatus{SpawnID: spawn.SpawnID}

		// Check if tracked object is in a player's weapon slot.
		if tracker.CurrentObjectID != 0xFFFF {
			if playerIdx, held := playerSlots[tracker.CurrentObjectID]; held {
				tracker.Status = "held"
				tracker.HeldBy = playerIdx
				status.Status = "held"
				status.HeldBy = &playerIdx
				result[idx] = status
				continue
			}
		}

		// Check world objects for this spawn's tag.
		worldObjs := worldByTag[spawn.Tag]
		if len(worldObjs) > 0 {
			var best *worldObj
			// Prefer the currently tracked object if still in world.
			for i := range worldObjs {
				if tracker.CurrentObjectID != 0xFFFF && worldObjs[i].objectID == tracker.CurrentObjectID {
					best = &worldObjs[i]
					break
				}
			}
			// Otherwise pick closest to spawn point.
			if best == nil {
				minDist := float32(math.MaxFloat32)
				for i := range worldObjs {
					dx := worldObjs[i].x - spawn.X
					dy := worldObjs[i].y - spawn.Y
					dz := worldObjs[i].z - spawn.Z
					d := dx*dx + dy*dy + dz*dz
					if d < minDist {
						minDist = d
						best = &worldObjs[i]
					}
				}
				tracker.CurrentObjectID = best.objectID
			}
			tracker.Status = "world"
			tracker.HeldBy = -1
			pos := &scraper.XYZ{X: best.x, Y: best.y, Z: best.z}
			status.Status = "world"
			status.WorldPos = pos
			result[idx] = status
			continue
		}

		// Not held, not in world → respawning.
		if tracker.Status == "respawning" {
			tracker.RespawnTimer--
			if tracker.RespawnTimer < 0 {
				tracker.RespawnTimer = 0
			}
		} else {
			tracker.Status = "respawning"
			tracker.HeldBy = -1
			tracker.CurrentObjectID = 0xFFFF
			tracker.RespawnTimer = int32(spawn.SpawnIntervalTicks)
		}
		rt := tracker.RespawnTimer
		status.Status = "respawning"
		status.RespawnInTicks = &rt
		result[idx] = status
	}
	return result
}

// -------------------------------------------------------------------
// Power item spawns (split: scenario-static positions + match-static OIDs)
// -------------------------------------------------------------------

// readPowerSpawnScenarios returns the scenario-static portion of every power
// item spawn — position, tag, respawn interval. Tag and interval are derived
// from tag-data which is map-static, so the whole result lives in
// scenarioStaticCache.PowerSpawnsScenario. The match-static InitialObjectID
// is filled separately by readPowerInitialOIDs once world objects exist.
//
// Returns nil if the scenario isn't loaded yet.
func (r *Reader) readPowerSpawnScenarios() []scenarioPowerSpawn {
	inst := r.inst
	mem := inst.Mem

	scenarioBase, err := inst.DerefLowPtr(AddrGlobalScenarioPtr)
	if err != nil || scenarioBase < HighGVAThreshold {
		return nil
	}
	if err := r.ensureBases(); err != nil {
		return nil
	}

	itemCount, _ := mem.ReadS32(scenarioBase + OffScenarioItemCount)
	if itemCount <= 0 {
		return nil
	}
	firstItemAddr, _ := mem.ReadU32(scenarioBase + OffScenarioItemFirst)
	if firstItemAddr < HighGVAThreshold {
		return nil
	}

	var spawns []scenarioPowerSpawn
	for i := int32(0); i < itemCount; i++ {
		itemAddr := firstItemAddr + uint32(i)*ScenarioItemStride
		tagIdxRaw, _ := mem.ReadS32(itemAddr + OffScenItemTagIndex)
		if tagIdxRaw < 0 {
			continue
		}
		tagIdx := int16(tagIdxRaw)
		tagName, err := r.readTagName(tagIdx)
		if err != nil || tagName == "" {
			continue
		}
		interval := r.readSpawnInterval(tagIdx)
		if interval <= 0 {
			continue // not a power item
		}
		x, _ := mem.ReadF32(itemAddr + OffScenItemX)
		y, _ := mem.ReadF32(itemAddr + OffScenItemY)
		z, _ := mem.ReadF32(itemAddr + OffScenItemZ)

		spawns = append(spawns, scenarioPowerSpawn{
			SpawnID:            len(spawns),
			Tag:                tagName,
			SpawnIntervalTicks: interval,
			X:                  x,
			Y:                  y,
			Z:                  z,
		})
	}
	return spawns
}

// readPowerInitialOIDs walks the world-object header once and finds the
// closest world object (by Euclidean distance) to each scenario power-spawn
// position with a matching tag index. The result is the per-match initial
// object ID used by power-item event detection. Returns an empty map when
// the world-object header isn't populated yet — caller should retry on the
// next ensureMatchStatic call.
func (r *Reader) readPowerInitialOIDs(spawns []scenarioPowerSpawn) map[int]uint32 {
	if len(spawns) == 0 || r.ohdBase < HighGVAThreshold {
		return nil
	}
	mem := r.inst.Mem

	objElemSize, _ := mem.ReadU16(r.ohdBase + OffOHDElementSize)
	objHeaderFirst, _ := mem.ReadU32(r.ohdBase + OffOHDFirstElement)
	objAllocCount, _ := mem.ReadU16(r.ohdBase + OffOHDAllocCount)
	if objHeaderFirst < HighGVAThreshold || objElemSize == 0 {
		return nil
	}

	type worldObj struct {
		objectID uint32
		tagIdx   int16
		x, y, z  float32
	}
	worldObjs := make([]worldObj, 0, objAllocCount)
	for i := uint16(0); i < objAllocCount; i++ {
		entryAddr := objHeaderFirst + uint32(i)*uint32(objElemSize)
		objDataAddr, _ := mem.ReadU32(entryAddr + OffObjEntryDataAddr)
		if objDataAddr < HighGVAThreshold {
			continue
		}
		flags, _ := mem.ReadU32(objDataAddr + OffObjFlags)
		if flags&ObjFlagGarbage != 0 {
			continue
		}
		tIdx, _ := mem.ReadS16(objDataAddr + OffObjTagIndex)
		x, _ := mem.ReadF32(objDataAddr + OffObjX)
		y, _ := mem.ReadF32(objDataAddr + OffObjY)
		z, _ := mem.ReadF32(objDataAddr + OffObjZ)
		worldObjs = append(worldObjs, worldObj{uint32(i), tIdx, x, y, z})
	}

	out := make(map[int]uint32, len(spawns))
	for _, sp := range spawns {
		spTagIdx, err := r.findTagIndex(sp.Tag)
		if err != nil || spTagIdx < 0 {
			out[sp.SpawnID] = 0xFFFF
			continue
		}
		initialOID := uint32(0xFFFF)
		minDist := float32(math.MaxFloat32)
		for _, wo := range worldObjs {
			if wo.tagIdx != spTagIdx {
				continue
			}
			dx, dy, dz := wo.x-sp.X, wo.y-sp.Y, wo.z-sp.Z
			d := dx*dx + dy*dy + dz*dz
			if d < minDist {
				minDist = d
				initialOID = wo.objectID
			}
		}
		out[sp.SpawnID] = initialOID
	}
	return out
}

// findTagIndex reverse-looks-up a tag index by its name string, scanning the
// already-populated tagNameCache. Returns -1 if the name hasn't been seen yet.
// Used by readPowerInitialOIDs when correlating scenario power-spawn tag
// names to world-object tag indices.
func (r *Reader) findTagIndex(name string) (int16, error) {
	for idx, n := range r.tagNameCache {
		if n == name {
			return idx, nil
		}
	}
	return -1, nil
}

func (r *Reader) readSpawnInterval(tagIdx int16) int16 {
	if r.tagInstBase < HighGVAThreshold {
		return 0
	}
	mem := r.inst.Mem
	tagInstEntry := r.tagInstBase + uint32(TagInstStride)*uint32(uint16(tagIdx))
	tagDataPtr, _ := mem.ReadU32(tagInstEntry + OffTagDataPtr)
	if tagDataPtr < HighGVAThreshold {
		return 0
	}
	intervalTablePtr, _ := mem.ReadU32(tagDataPtr + OffTagRespawnIntervalOff)
	if intervalTablePtr < HighGVAThreshold {
		return 0
	}
	interval, _ := mem.ReadS16(intervalTablePtr + OffTagRespawnInterval)
	return interval
}

// -------------------------------------------------------------------
// Tag name lookup
// -------------------------------------------------------------------

func (r *Reader) readTagName(tagIdx int16) (string, error) {
	if name, ok := r.tagNameCache[tagIdx]; ok {
		return name, nil
	}
	if r.tagInstBase < HighGVAThreshold {
		return "", nil
	}
	mem := r.inst.Mem
	tagInstEntry := r.tagInstBase + uint32(TagInstStride)*uint32(uint16(tagIdx))
	namePtr, err := mem.ReadU32(tagInstEntry + OffTagNamePtr)
	if err != nil || namePtr < HighGVAThreshold {
		return "", err
	}
	name := r.readHighString(namePtr)
	r.tagNameCache[tagIdx] = name
	return name, nil
}

func (r *Reader) readHighString(gva uint32) string {
	if gva < HighGVAThreshold {
		return ""
	}
	b, err := r.inst.Mem.ReadBytes(gva, 128)
	if err != nil {
		return ""
	}
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

func (r *Reader) ensureBases() error {
	if r.tagInstBase == 0 {
		base, err := r.inst.DerefLowPtr(AddrGlobalTagInstancesPtr)
		if err != nil {
			return err
		}
		r.tagInstBase = base
	}
	if r.ohdBase == 0 {
		base, err := r.inst.DerefLowPtr(AddrObjectHeaderDatumPtr)
		if err != nil {
			return err
		}
		r.ohdBase = base
	}
	return nil
}

// readLowString reads a null-terminated ASCII string from a cached low GVA host VA.
func (r *Reader) readLowString(lowGVA uint32, maxLen int) string {
	hva, err := r.inst.LowHVA(lowGVA)
	if err != nil {
		return ""
	}
	b, err := r.inst.Mem.ReadBytesAt(hva, maxLen)
	if err != nil {
		return ""
	}
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// decodeUTF16LE decodes a null-terminated UTF-16LE byte slice into a Go string.
func decodeUTF16LE(b []byte) string {
	u16s := make([]uint16, len(b)/2)
	for i := range u16s {
		u16s[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	for i, c := range u16s {
		if c == 0 {
			u16s = u16s[:i]
			break
		}
	}
	return string(utf16.Decode(u16s))
}
