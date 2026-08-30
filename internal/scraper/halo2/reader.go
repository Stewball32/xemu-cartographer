package halo2

import (
	"fmt"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/xbox"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// Reader reads live Halo 2 state from one xemu instance via the W1-W4 offset map.
type Reader struct {
	inst *xemu.Instance
	name string
	// off is the instance's versioned address layer (baseline for stock H2,
	// or an assigned offset set for a modded build).
	off  Offsets
	tick uint32

	lastInputs scraper.StateInputs
}

// NewReader constructs a Halo 2 Reader.
func NewReader(inst *xemu.Instance, instanceName string, off Offsets) *Reader {
	return &Reader{inst: inst, name: instanceName, off: off, lastInputs: scraper.StateInputs{}}
}

// validHigh reports whether p is a plausible high-GVA heap pointer.
func validHigh(p uint32) bool { return p >= 0x80000000 && p != 0xFFFFFFFF }

// ---------------------------------------------------------------------------
// Low-global VALUE reads. DerefLowPtr covers "read the u32 at a low GVA"; the
// per-slot stat arrays additionally need sized reads at base+delta. Deltas
// must stay inside the base's 4K page (the translation is per-page) — the
// slot math in stats.go keeps them there for all 16 player slots.
// ---------------------------------------------------------------------------

func (r *Reader) lowU16(baseGVA uint32, delta int64) (uint16, bool) {
	hva, err := r.inst.LowHVA(baseGVA)
	if err != nil {
		return 0, false
	}
	v, err := r.inst.Mem.ReadU16At(hva + delta)
	return v, err == nil
}

func (r *Reader) lowU32(baseGVA uint32, delta int64) (uint32, bool) {
	hva, err := r.inst.LowHVA(baseGVA)
	if err != nil {
		return 0, false
	}
	v, err := r.inst.Mem.ReadU32At(hva + delta)
	return v, err == nil
}

func (r *Reader) lowU8(baseGVA uint32, delta int64) (uint8, bool) {
	hva, err := r.inst.LowHVA(baseGVA)
	if err != nil {
		return 0, false
	}
	v, err := r.inst.Mem.ReadU8At(hva + delta)
	return v, err == nil
}

func (r *Reader) lowBytes(baseGVA uint32, delta int64, n int) ([]byte, bool) {
	hva, err := r.inst.LowHVA(baseGVA)
	if err != nil {
		return nil, false
	}
	b, err := r.inst.Mem.ReadBytesAt(hva+delta, n)
	return b, err == nil
}

// arrayInfo is the decoded s_data_array header.
type arrayInfo struct {
	base     uint32 // high-GVA of the data_array
	sig      string
	max      uint32
	elemSize uint32
	active   uint32
	block    uint32 // high-GVA of the element block
	ok       bool
}

// readArray derefs a static .data pointer and decodes the data-array header.
func (r *Reader) readArray(lowPtr uint32) (arrayInfo, error) {
	base, err := r.inst.DerefLowPtr(lowPtr)
	if err != nil {
		return arrayInfo{}, fmt.Errorf("deref 0x%x: %w", lowPtr, err)
	}
	ai := arrayInfo{base: base}
	if !validHigh(base) {
		return ai, nil
	}
	sigB, err := r.inst.Mem.ReadBytes(base+r.off.OffH2DataArraySignature, 4)
	if err != nil {
		return ai, nil
	}
	ai.sig = string(sigB)
	ai.max, _ = r.inst.Mem.ReadU32(base + r.off.OffH2DataArrayMax)
	ai.elemSize, _ = r.inst.Mem.ReadU32(base + r.off.OffH2DataArrayElemSize)
	ai.active, _ = r.inst.Mem.ReadU32(base + r.off.OffH2DataArrayActiveCount)
	ai.block, _ = r.inst.Mem.ReadU32(base + r.off.OffH2DataArrayBlockPtr)
	ai.ok = ai.sig == DataArraySignature && validHigh(ai.block)
	return ai, nil
}

// resolveObject maps a Halo handle to its object-data high-GVA via the object
// entry table. Returns 0 if the handle is null or unresolvable.
func (r *Reader) resolveObject(objs arrayInfo, handle uint32) uint32 {
	if !objs.ok || handle == 0 || handle == 0xFFFFFFFF {
		return 0
	}
	idx := handle & 0xFFFF
	if idx >= objs.max {
		return 0
	}
	entry := objs.block + idx*ConstH2ObjElemSize
	data, err := r.inst.Mem.ReadU32(entry + r.off.OffH2ObjEntryDataPtr)
	if err != nil || !validHigh(data) {
		return 0
	}
	return data
}

// rosterEntry is an internal decode of one active player datum.
type rosterEntry struct {
	slot       int
	index      int32
	team       int32
	name       string
	unit       uint32
	machineIdx int   // owning machine's session index (system link)
	macOctet   uint8 // stable player id — survives H2's dup-name renames
	betrayals  uint16
}

// readRoster iterates the players array and returns active player datums.
func (r *Reader) readRoster(players arrayInfo) []rosterEntry {
	if !players.ok {
		return nil
	}
	max := players.max
	if max == 0 || max > ConstH2PlayerMax {
		max = ConstH2PlayerMax
	}
	out := make([]rosterEntry, 0, max)
	for i := uint32(0); i < max; i++ {
		rec := players.block + i*ConstH2PlayerRecordSize
		did, err := r.inst.Mem.ReadU32(rec + r.off.OffH2PlrDatumId)
		if err != nil || did == 0 || did == 0xFFFFFFFF {
			continue
		}
		idx, _ := r.inst.Mem.ReadS32(rec + r.off.OffH2PlrIndex)
		team, _ := r.inst.Mem.ReadS32(rec + r.off.OffH2PlrTeam)
		unit, _ := r.inst.Mem.ReadU32(rec + r.off.OffH2PlrUnitHandle)
		nameB, _ := r.inst.Mem.ReadBytes(rec+r.off.OffH2PlrName, 32)
		machineIdx, _ := r.inst.Mem.ReadU8(rec + r.off.OffH2PlrMachineIndex)
		macOctet, _ := r.inst.Mem.ReadU8(rec + r.off.OffH2PlrMacOctet)
		betrayals, _ := r.inst.Mem.ReadU16(rec + r.off.OffH2PlrBetrayals)
		out = append(out, rosterEntry{
			slot:       int(i),
			index:      idx,
			team:       team,
			name:       xbox.DecodeUTF16LEBounded(nameB, 16),
			unit:       unit,
			machineIdx: int(machineIdx),
			macOctet:   macOctet,
			betrayals:  betrayals,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// GameReader implementation
// ---------------------------------------------------------------------------

// ReadGameState classifies the engine lifecycle. When the bound set maps the
// lifecycle enum (AddrH2GamePhase — Slim builds; full-cycle validated
// menu→lobby→in-game→postgame→lobby), it is the authority and yields the full
// 4-state including pregame/postgame. On builds without it (stock: 0
// sentinel), fall back to the array inference: in-game requires the players
// array valid+active and at least one player's unit handle to resolve to a
// biped with a plausible health fraction.
func (r *Reader) ReadGameState() (scraper.GameState, uint32, error) {
	players, err := r.readArray(r.off.AddrH2PlayersArrayPtr)
	if err != nil {
		return scraper.GameStateMenu, r.tick, err // genuine read failure
	}
	objs, _ := r.readArray(r.off.AddrH2ObjectArrayPtr)

	inputs := scraper.StateInputs{
		"players_ptr":    fmt.Sprintf("0x%x", players.base),
		"players_sig":    players.sig,
		"players_active": players.active,
		"objects_active": objs.active,
	}

	if r.off.AddrH2GamePhase != 0 {
		if raw, ok := r.lowU32(r.off.AddrH2GamePhase, 0); ok {
			inputs["phase_enum"] = raw
			if gs, known := phaseFromEnum(raw); known {
				inputs["in_game"] = gs == scraper.GameStateInGame
				r.lastInputs = inputs
				if gs == scraper.GameStateInGame {
					r.tick++
				}
				return gs, r.tick, nil
			}
		}
	}

	inGame := false
	if players.ok && players.active >= 1 {
		for _, p := range r.readRoster(players) {
			od := r.resolveObject(objs, p.unit)
			if od == 0 {
				continue
			}
			if h, err := r.inst.Mem.ReadF32(od + r.off.OffH2BipedHealth); err == nil && h >= -0.01 && h <= 2.0 {
				inGame = true
				break
			}
		}
	}
	inputs["in_game"] = inGame
	r.lastInputs = inputs

	if inGame {
		r.tick++
		return scraper.GameStateInGame, r.tick, nil
	}
	return scraper.GameStateMenu, r.tick, nil
}

func (r *Reader) LastStateInputs() scraper.StateInputs { return r.lastInputs }

// BuildScoreProbe surfaces the raw stats/config globals for the debug page:
// the gametype enum, per-slot K/D reads, the match kill total, and (when the
// build maps it) the lifecycle enum.
func (r *Reader) BuildScoreProbe() scraper.ScoreProbe {
	probe := scraper.ScoreProbe{}
	if th, err := r.inst.DerefLowPtr(r.off.AddrH2TagHeaderPtr); err == nil && validHigh(th) {
		scen, _ := r.inst.Mem.ReadU32(th + r.off.OffH2TagHdrScenarioId)
		probe["scenario_id"] = fmt.Sprintf("0x%x", scen)
	}
	if v, ok := r.lowU32(r.off.AddrH2Gametype, 0); ok {
		probe["gametype_enum"] = v
		probe["gametype"] = gametypeName(v)
	}
	if v, ok := r.lowU32(r.off.AddrH2KillsTotal, 0); ok {
		probe["kills_total"] = v
	}
	for slot := 0; slot < 4; slot++ {
		if k, ok := r.lowU16(r.off.AddrH2KillsPerPlayer, killsSlotDelta(slot)); ok {
			probe[fmt.Sprintf("kills_slot%d", slot)] = k
		}
		if d, ok := r.lowU32(r.off.AddrH2DeathsPerPlayer, deathsSlotDelta(slot)); ok {
			probe[fmt.Sprintf("deaths_slot%d", slot)] = d
		}
	}
	if r.off.AddrH2GamePhase != 0 {
		if v, ok := r.lowU32(r.off.AddrH2GamePhase, 0); ok {
			probe["phase_enum"] = v
		}
	} else {
		probe["phase_enum"] = "unmapped on this build (stock)"
	}
	probe["note"] = "assists / score / kill-streak have no known H2 offsets on any build; team scores underived"
	return probe
}

// ReadGameData reads match config + roster: real per-player kills/deaths from
// the per-slot stat arrays (K/D semantics resolved 2026-07-11), the gametype
// enum, per-player betrayals (LOCAL-ONLY in system link — remote players'
// betrayals undercount on a host scraper), and the system-link machine layer
// (per-player machine index + is_local against this console's own index).
// Assists / score / kill-streak have NO known H2 offsets on any build and stay
// zero — honestly absent rather than guessed.
func (r *Reader) ReadGameData() (scraper.GameData, error) {
	players, err := r.readArray(r.off.AddrH2PlayersArrayPtr)
	if err != nil {
		return scraper.GameData{}, err
	}
	roster := r.readRoster(players)

	gametype := ""
	if v, ok := r.lowU32(r.off.AddrH2Gametype, 0); ok {
		gametype = gametypeName(v)
	}

	// This console's machine index — the is_local pivot. In a local
	// (non-link) game the roster region is zeroed and every player's
	// machineIdx is 0 == localIdx 0, which is correct: they ARE all local.
	localIdx := -1
	if v, ok := r.lowU8(r.off.AddrH2NetLocalMachineIndex, 0); ok {
		localIdx = int(v)
	}

	gd := scraper.GameData{
		Map:        r.mapName(),
		Gametype:   gametype,
		TeamScores: []scraper.TeamScore{},
		Machines:   r.readMachines(localIdx),
	}
	teams := map[uint32]struct{}{}
	localCount := 0
	for _, p := range roster {
		gp := scraper.GamePlayer{
			Index: int(p.index),
			Name:  p.name,
			Team:  uint32(p.team),
			// Assists deliberately absent — no offset exists (any build).
			TeamKills: int16(p.betrayals),
		}
		if k, ok := r.lowU16(r.off.AddrH2KillsPerPlayer, killsSlotDelta(p.slot)); ok {
			gp.Kills = int16(k)
		}
		if d, ok := r.lowU32(r.off.AddrH2DeathsPerPlayer, deathsSlotDelta(p.slot)); ok {
			gp.Deaths = int16(d)
		}
		if localIdx >= 0 {
			mi := p.machineIdx
			isLocal := mi == localIdx
			gp.MachineIndex = &mi
			gp.IsLocal = &isLocal
			if isLocal {
				li := localCount
				gp.LocalIndex = &li
				localCount++
			}
		}
		gd.Players = append(gd.Players, gp)
		teams[uint32(p.team)] = struct{}{}
	}
	gd.LocalCount = uint16(localCount)
	if localIdx < 0 {
		// No machine layer readable — fall back to the old "everyone active"
		// count so LocalCount stays meaningful for splitscreen display.
		gd.LocalCount = uint16(players.active)
	}
	gd.IsTeamGame = len(teams) > 1
	return gd, nil
}

// readMachines decodes the system-link machine roster: MACs from the MAC
// array (stride 6, non-zero = present), names from the machine table
// (page-guarded — see machineNameReadable). Empty outside a link session.
func (r *Reader) readMachines(localIdx int) []scraper.GameMachine {
	var out []scraper.GameMachine
	for i := 0; i < int(ConstH2NetMachineMax); i++ {
		mac, ok := r.lowBytes(r.off.AddrH2NetMachineMacArray, macSlotDelta(i), 6)
		if !ok {
			break
		}
		nonZero := false
		for _, b := range mac {
			if b != 0 {
				nonZero = true
				break
			}
		}
		if !nonZero {
			continue
		}
		name := ""
		if machineNameReadable(r.off.AddrH2NetMachineTable, i, 64) {
			if b, ok := r.lowBytes(r.off.AddrH2NetMachineTable,
				machineEntryDelta(i)+int64(r.off.OffH2NetMachineName), 64); ok {
				name = xbox.DecodeUTF16LEBounded(b, 32)
			}
		}
		isLocal := i == localIdx
		out = append(out, scraper.GameMachine{Index: i, Name: name, IsLocal: &isLocal})
	}
	return out
}

// ReadReadyState is the cheap Ready-phase variant; same shape as ReadGameData.
func (r *Reader) ReadReadyState() (scraper.GameData, error) { return r.ReadGameData() }

// ReadTick reads per-tick volatile biped state for every rostered player.
func (r *Reader) ReadTick(_ []scraper.PowerItemSpawn, _ *scraper.TickState) (scraper.TickResult, error) {
	players, err := r.readArray(r.off.AddrH2PlayersArrayPtr)
	if err != nil {
		return scraper.TickResult{}, err
	}
	objs, _ := r.readArray(r.off.AddrH2ObjectArrayPtr)
	roster := r.readRoster(players)

	var tr scraper.TickResult
	tps := make([]scraper.TickPlayer, 0, len(roster))
	for _, p := range roster {
		tp := scraper.TickPlayer{Index: int(p.index)}
		od := r.resolveObject(objs, p.unit)
		if od != 0 {
			health, _ := r.inst.Mem.ReadF32(od + r.off.OffH2BipedHealth)
			shield, _ := r.inst.Mem.ReadF32(od + r.off.OffH2BipedShield)
			maxH, _ := r.inst.Mem.ReadF32(od + r.off.OffH2BipedMaxHealth)
			maxS, _ := r.inst.Mem.ReadF32(od + r.off.OffH2BipedMaxShield)
			px, _ := r.inst.Mem.ReadF32(od + r.off.OffH2ObjPosition)
			py, _ := r.inst.Mem.ReadF32(od + r.off.OffH2ObjPosition + 4)
			pz, _ := r.inst.Mem.ReadF32(od + r.off.OffH2ObjPosition + 8)
			ax, _ := r.inst.Mem.ReadF32(od + r.off.OffH2ObjAim)
			ay, _ := r.inst.Mem.ReadF32(od + r.off.OffH2ObjAim + 4)
			az, _ := r.inst.Mem.ReadF32(od + r.off.OffH2ObjAim + 8)
			vx, _ := r.inst.Mem.ReadF32(od + r.off.OffH2ObjVelocity)
			vy, _ := r.inst.Mem.ReadF32(od + r.off.OffH2ObjVelocity + 4)
			vz, _ := r.inst.Mem.ReadF32(od + r.off.OffH2ObjVelocity + 8)
			frags, _ := r.inst.Mem.ReadU8(od + r.off.OffH2BipedFragGrenades)
			plasmas, _ := r.inst.Mem.ReadU8(od + r.off.OffH2BipedPlasmaGrenades)
			slot, _ := r.inst.Mem.ReadU8(od + r.off.OffH2BipedCurWeaponSlot)

			tp.Alive = health > 0
			tp.Health, tp.Shields = health, shield
			tp.MaxHealth, tp.MaxShields = maxH, maxS
			tp.X, tp.Y, tp.Z = px, py, pz
			tp.AimX, tp.AimY, tp.AimZ = ax, ay, az
			tp.VX, tp.VY, tp.VZ = vx, vy, vz
			tp.Frags, tp.Plasmas = frags, plasmas
			tp.SelectedWeaponSlot = int16(slot)
			tp.Weapons = r.readWeapons(objs, od)
		}
		tps = append(tps, tp)
	}
	tr.Payload.Players = tps
	tr.Payload.PlayerCount = int16(len(tps))
	tr.Payload.LocalCount = uint16(players.active)
	return tr, nil
}

// readWeapons reads the biped's 4 weapon slots and their ammo.
func (r *Reader) readWeapons(objs arrayInfo, biped uint32) []scraper.WeaponInfo {
	var out []scraper.WeaponInfo
	for slot := uint32(0); slot < 4; slot++ {
		h, err := r.inst.Mem.ReadU32(biped + r.off.OffH2BipedWeaponSlots + slot*4)
		if err != nil || h == 0 || h == 0xFFFFFFFF {
			continue
		}
		wd := r.resolveObject(objs, h)
		if wd == 0 {
			continue
		}
		mag, _ := r.inst.Mem.ReadU16(wd + r.off.OffH2WepMag)
		res, _ := r.inst.Mem.ReadU16(wd + r.off.OffH2WepReserve)
		magI, resI := int16(mag), int16(res)
		out = append(out, scraper.WeaponInfo{
			Slot:     int(slot),
			ObjectID: h & 0xFFFF,
			AmmoMag:  &magI,
			AmmoPack: &resI,
		})
	}
	return out
}

// DetectEvents returns no events yet: the H2 event buffer is non-functional
// (LegacyGVAEventCount always reads 0) and the stat-diff fallback needs the
// K/D/A stats block, which does not resolve on this build. M20 known-broken.
func (r *Reader) DetectEvents(_ uint32, _ string, _ scraper.GameData, _ scraper.TickResult, _ *scraper.TickState) []scraper.Envelope {
	return nil
}

// OnStateChange has no scenario/match caches to invalidate yet.
func (r *Reader) OnStateChange(_ scraper.GameState, _ scraper.GameState) error { return nil }

// mapName resolves the loaded scenario to a display name. Primary path: the
// scenario tag-name pool carries the loaded scenario's path string at
// +OffH2ScenarioPathInPool (e.g. scenarios\multi\zanzibar\zanzibar) — decode
// it, take the basename, and map through the display table. This works for
// EVERY map, DLC included, unlike the previous known-scenario-id lookup, which
// stays as the fallback.
func (r *Reader) mapName() string {
	if name := r.mapNameFromPool(); name != "" {
		return name
	}
	th, err := r.inst.DerefLowPtr(r.off.AddrH2TagHeaderPtr)
	if err != nil || !validHigh(th) {
		return ""
	}
	scen, err := r.inst.Mem.ReadU32(th + r.off.OffH2TagHdrScenarioId)
	if err != nil {
		return ""
	}
	if internal, ok := scenarioIDNames[scen]; ok {
		if disp, ok := mapDisplayNames[internal]; ok {
			return disp
		}
		return internal
	}
	return fmt.Sprintf("scnr:0x%08x", scen)
}

// mapNameFromPool reads the scenario path out of the tag-name pool and turns
// its basename into a display name ("" when the pool isn't up or the string
// doesn't look like a tag path).
func (r *Reader) mapNameFromPool() string {
	pool, err := r.inst.DerefLowPtr(r.off.AddrH2ScenarioNamePoolPtr)
	if err != nil || !validHigh(pool) {
		return ""
	}
	raw, err := r.inst.Mem.ReadBytes(pool+r.off.OffH2ScenarioPathInPool, 96)
	if err != nil {
		return ""
	}
	internal, ok := scenarioBasename(raw)
	if !ok {
		return ""
	}
	if disp, ok := mapDisplayNames[internal]; ok {
		return disp
	}
	return internal
}
