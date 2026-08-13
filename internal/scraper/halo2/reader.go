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
	slot  int
	index int32
	team  int32
	name  string
	unit  uint32
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
		out = append(out, rosterEntry{
			slot:  int(i),
			index: idx,
			team:  team,
			name:  xbox.DecodeUTF16LEBounded(nameB, 16),
			unit:  unit,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// GameReader implementation
// ---------------------------------------------------------------------------

// ReadGameState classifies menu vs in-game. In-game requires the players array
// to be valid+active and at least one player's unit handle to resolve to a
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

// BuildScoreProbe surfaces the raw score/gametype candidates. H2 gametype +
// team scores are not yet in the verified offset map (legacy globals stale);
// this reports the scenario id so the debug page can key maps until re-derived.
func (r *Reader) BuildScoreProbe() scraper.ScoreProbe {
	probe := scraper.ScoreProbe{}
	if th, err := r.inst.DerefLowPtr(r.off.AddrH2TagHeaderPtr); err == nil && validHigh(th) {
		scen, _ := r.inst.Mem.ReadU32(th + r.off.OffH2TagHdrScenarioId)
		probe["scenario_id"] = fmt.Sprintf("0x%x", scen)
	}
	probe["note"] = "gametype/team-score offsets unverified (legacy globals stale) — M20 re-derivation pending"
	return probe
}

// ReadGameData reads match config + roster. Map is keyed off the scenario id;
// gametype and K/D/A are stubbed (legacy stats-block globals do not resolve on
// this build — M20 known-broken).
func (r *Reader) ReadGameData() (scraper.GameData, error) {
	players, err := r.readArray(r.off.AddrH2PlayersArrayPtr)
	if err != nil {
		return scraper.GameData{}, err
	}
	roster := r.readRoster(players)

	gd := scraper.GameData{
		Map:        r.mapName(),
		Gametype:   "", // UNVERIFIED — legacy gametype global stale (M20)
		TeamScores: []scraper.TeamScore{},
		LocalCount: uint16(players.active),
	}
	teams := map[uint32]struct{}{}
	for _, p := range roster {
		gd.Players = append(gd.Players, scraper.GamePlayer{
			Index: int(p.index),
			Name:  p.name,
			Team:  uint32(p.team),
			// K/D/A stubbed: stats-block globals do not resolve (M20).
		})
		teams[uint32(p.team)] = struct{}{}
	}
	gd.IsTeamGame = len(teams) > 1
	return gd, nil
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

// mapName resolves the loaded scenario to a display name via the tag header's
// scenario id. Full scenario-path string decoding (tag-name pool) is a pending
// follow-up; until then known ids map to names, else "scnr:0x........".
func (r *Reader) mapName() string {
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
