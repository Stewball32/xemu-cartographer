package manager

import (
	"encoding/json"
	"math"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// v2 reader-to-payload adapters. Convert the existing v1 reader output
// (scraper.GameData, scraper.TickPayload, scraper.WeaponInfo, etc.) into
// the per-class v2 payload structs (XboxPayload, ScenarioPayload,
// GamePayload, TickPayload, ObjectsPayload, DebugPayload,
// PreviousGamePayload) defined in xbox.go / scenario.go / game.go /
// tick.go / objects.go / debug.go / previous_game.go.
//
// PR 8 of the v2 rebuild — this PR ONLY adds the translation layer; the
// runner still emits the v1-style envelopes. PR 9 wires these adapters
// into the runner's emission path and switches to per-class room routing.
//
// All adapters take a snapshot copy of instanceCache so the caller
// (runner) does not have to hold cacheMu through the build.
//
// Optional payloads (Scenario / Tick / Objects / Debug / PreviousGame)
// return nil when the underlying cache state isn't populated; the caller
// skips emission of those classes when nil.

// --- helpers ---

const handleEmpty32 uint32 = 0xFFFFFFFF

// optUint32 returns nil when v is the "no object" sentinel (0xFFFFFFFF),
// else a pointer to v. Used wherever Halo CE stores an absent object
// handle as 0xFFFFFFFF and v2 wants a nullable pointer.
func optUint32(v uint32) *uint32 {
	if v == handleEmpty32 {
		return nil
	}
	return &v
}

// safeFloat replaces NaN / ±Inf with 0. encoding/json refuses to marshal
// either and returns an error; in our pipeline that error gets dropped in
// MakeEnvelope and the resulting envelope wire-bytes show `data: null` —
// a silent payload-loss bug we hit in PR 9 verification when bones
// included NaN floats (Halo CE memory regions for bones contain NaN for
// some player states: just-spawned, in-vehicle, etc.).
//
// Sanitizing at the adapter layer keeps the bug from propagating to the
// wire. Real values pass through unchanged; junk values become explicit
// zeros that consumers can recognise (vs the all-or-nothing data: null
// failure mode).
func safeFloat(f float32) float32 {
	d := float64(f)
	if math.IsNaN(d) || math.IsInf(d, 0) {
		return 0
	}
	return f
}

// safeMapAny returns a copy of m with un-marshalable values dropped.
// Used to sanitize the StateInputs / ScoreProbe map[string]any fields
// in ProbePayload — those carry plugin-defined typed structs whose
// float fields may contain NaN from uninitialised memory reads. One
// NaN in one nested probe value would otherwise null the whole probe
// payload (same root cause as safeFloat, different shape — we can't
// recurse into arbitrary struct types without reflection).
//
// Returns nil if m is nil. Dropped keys are silently omitted — the
// probe envelope's job is best-effort, not lossless.
func safeMapAny(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if _, err := json.Marshal(v); err != nil {
			continue
		}
		out[k] = v
	}
	return out
}

// vec3 builds a Vec3 from flat float32s, sanitizing each component.
func vec3(x, y, z float32) scraper.Vec3 {
	return scraper.Vec3{X: safeFloat(x), Y: safeFloat(y), Z: safeFloat(z)}
}

// vec3PtrFromXYZ converts *scraper.XYZ to *Vec3 (nil-safe, sanitized).
func vec3PtrFromXYZ(p *scraper.XYZ) *scraper.Vec3 {
	if p == nil {
		return nil
	}
	v := vec3(p.X, p.Y, p.Z)
	return &v
}

// --- xbox ---

// buildXboxPayload extracts the console/identity fields from an
// instanceCache snapshot. The three nested objects (TimeZone, XBE,
// Kernel) are populated only when the runner's system snapshot has
// supplied real values; the object's presence is the signal "read",
// not any one field's value. This is the fix for v1's omitempty
// zero-value-drop bug.
func buildXboxPayload(c *instanceCache) XboxPayload {
	p := XboxPayload{
		TitleID:       c.TitleID,
		Title:         c.Title,
		Name:          c.XboxName,
		SerialNumber:  c.SerialNumber,
		MACAddress:    c.MACAddress,
		VideoStandard: c.VideoStandard,
	}
	// TimeZone: StdName is the gate — set by xbox.ReadTimeZone only on
	// successful read. bias=0 (GMT) is a real value and survives.
	if c.TimeZoneStdName != "" {
		p.TimeZone = &XboxTimeZone{
			BiasMinutes: c.TimeZoneBias,
			StdName:     c.TimeZoneStdName,
			DltName:     c.TimeZoneDltName,
		}
	}
	// XBE: TitleName is the gate. Zero-valued numerics (DiskNumber=0)
	// are real and survive.
	if c.XBETitleName != "" {
		p.XBE = &XboxXBE{
			TitleName:    c.XBETitleName,
			Version:      c.XBEVersion,
			GameRegion:   c.XBEGameRegion,
			DiskNumber:   c.XBEDiskNumber,
			AllowedMedia: c.XBEAllowedMedia,
		}
	}
	// Kernel: SystemTime non-zero is the gate. UptimeSeconds is float
	// for readability vs the 14-digit nanosecond int v1 shipped.
	if !c.KernelSystemTime.IsZero() {
		p.Kernel = &XboxKernel{
			SystemTime:    c.KernelSystemTime,
			BootTime:      c.KernelBootTime,
			UptimeSeconds: c.KernelUptime.Seconds(),
		}
	}
	return p
}

// --- scenario ---

// buildScenarioPayload returns nil when no scenario is loaded
// (cache.GameData == nil — runner is Idle). TagDefs is populated from
// the latest tick's per-player weapon tag_data; biped tag_defs are not
// yet wired (the v1 reader doesn't expose the biped tag string).
func buildScenarioPayload(c *instanceCache) *ScenarioPayload {
	if c.GameData == nil {
		return nil
	}
	gd := c.GameData
	p := &ScenarioPayload{
		Map:             gd.Map,
		GameDifficulty:  gd.GameDifficulty,
		ObjectTypes:     make([]ScenarioObjectType, 0, len(gd.ObjectTypes)),
		PlayerSpawns:    make([]ScenarioPlayerSpawn, 0, len(gd.PlayerSpawns)),
		PowerItemSpawns: make([]ScenarioPowerItemSpawn, 0, len(gd.PowerItemSpawns)),
		TagDefs:         map[string]ScenarioTagDef{},
	}
	if gd.Fog != nil {
		p.Fog = &ScenarioFog{
			Color: ScenarioFogColor{
				R: gd.Fog.ColorR,
				G: gd.Fog.ColorG,
				B: gd.Fog.ColorB,
			},
			MaxDensity:  gd.Fog.MaxDensity,
			AtmoMinDist: gd.Fog.AtmoMinDist,
			AtmoMaxDist: gd.Fog.AtmoMaxDist,
		}
	}
	if gd.TagCache != nil {
		tc := gd.TagCache
		p.MemoryRegions = &ScenarioMemoryRegions{
			GameState:    ScenarioMemoryRegion{Base: tc.GameStateBase, Size: tc.GameStateSize},
			TagCache:     ScenarioMemoryRegion{Base: tc.TagCacheBase, Size: tc.TagCacheSize},
			TextureCache: ScenarioMemoryRegion{Base: tc.TextureCacheBase, Size: tc.TextureCacheSize},
			SoundCache:   ScenarioMemoryRegion{Base: tc.SoundCacheBase, Size: tc.SoundCacheSize},
		}
	}
	for _, ot := range gd.ObjectTypes {
		p.ObjectTypes = append(p.ObjectTypes, ScenarioObjectType{
			TypeIndex: ot.TypeIndex,
			Name:      ot.Name,
			DatumSize: ot.DatumSize,
		})
	}
	for _, ps := range gd.PlayerSpawns {
		p.PlayerSpawns = append(p.PlayerSpawns, ScenarioPlayerSpawn{
			Index:     ps.Index,
			Pos:       vec3(ps.X, ps.Y, ps.Z),
			Facing:    ps.Facing,
			TeamIndex: ps.TeamIndex,
			BspIndex:  ps.BspIndex,
			Gametypes: [4]uint8{ps.Gametype0, ps.Gametype1, ps.Gametype2, ps.Gametype3},
		})
	}
	for _, pis := range gd.PowerItemSpawns {
		p.PowerItemSpawns = append(p.PowerItemSpawns, ScenarioPowerItemSpawn{
			SpawnID:       pis.SpawnID,
			Tag:           pis.Tag,
			IntervalTicks: pis.SpawnIntervalTicks,
			GametypeMask:  pis.GametypeMask,
			Pos:           vec3(pis.X, pis.Y, pis.Z),
		})
	}
	// Collect weapon tag_defs from the latest tick's per-player weapons.
	if c.LatestTick != nil {
		for _, tp := range c.LatestTick.Players {
			for _, w := range tp.Weapons {
				if w.TagData == nil {
					continue
				}
				if _, exists := p.TagDefs[w.Tag]; exists {
					continue
				}
				td := w.TagData
				p.TagDefs[w.Tag] = ScenarioTagDef{
					Kind:           "weapon",
					ZoomLevels:     td.ZoomLevels,
					ZoomMin:        td.ZoomMin,
					ZoomMax:        td.ZoomMax,
					AutoaimAngle:   td.AutoaimAngle,
					AutoaimRange:   td.AutoaimRange,
					MagnetismAngle: td.MagnetismAngle,
					MagnetismRange: td.MagnetismRange,
					DeviationAngle: td.DeviationAngle,
				}
			}
		}
	}
	return p
}

// --- game ---

// buildGamePayload assembles the v2 game class — phase, freshness, and
// (when not Idle) match state. Network info comes from cache.LatestTick
// (not cache.GameData) because the v1 reader places it under the tick
// payload; v2 moves it onto the game class since it's slow-changing.
func buildGamePayload(c *instanceCache) GamePayload {
	p := GamePayload{
		Phase:      c.Phase,
		StartedAt:  c.StartedAt,
		LastReadAt: c.LastReadAt,
		EngineTick: c.EngineTick,
		Iterations: c.Iterations,
		HostHealth: c.HostHealth,
		// Arrays start empty (not nil) so JSON marshals as [].
		TeamScores: []GameTeamScore{},
		Players:    []GameRosterPlayer{},
		Machines:   []GameMachine{},
	}
	if c.GameData != nil {
		gd := c.GameData
		p.GameState = string(gd.GameState)
		p.GameElapsedTicks = gd.ElapsedTicks
		p.Config = &GameConfig{
			Gametype:       gd.Gametype,
			VariantName:    gd.VariantName,
			IsTeamGame:     gd.IsTeamGame,
			ScoreLimit:     gd.ScoreLimit,
			TimeLimitTicks: gd.TimeLimitTicks,
		}
		for _, ts := range gd.TeamScores {
			p.TeamScores = append(p.TeamScores, GameTeamScore{Team: ts.Team, Score: ts.Score})
		}
		for _, pl := range gd.Players {
			// Merge the accumulated match stats (cache.PlayerAccum snapshot) by
			// player index; zero value when no accumulator ran yet (lobby/idle).
			acc := c.PlayerAccum[pl.Index]
			p.Players = append(p.Players, GameRosterPlayer{
				Index:           pl.Index,
				Name:            pl.Name,
				Team:            pl.Team,
				ArmorColor:      pl.ArmorColor,
				Score:           pl.Score,
				Kills:           pl.Kills,
				Deaths:          pl.Deaths,
				Assists:         pl.Assists,
				CTFScore:        pl.CTFScore,
				TeamKills:       pl.TeamKills,
				Suicides:        pl.Suicides,
				KillStreak:      pl.KillStreak,
				Multikill:       pl.Multikill,
				ShotsFired:      pl.ShotsFired,
				ShotsHit:        pl.ShotsHit,
				IsLocal:         pl.IsLocal,
				LocalIndex:      pl.LocalIndex,
				MachineIndex:    pl.MachineIndex,
				ControllerIndex: pl.ControllerIndex,

				AccShotsFired:     acc.ShotsFired,
				AccGrenadeThrows:  acc.GrenadeThrows,
				AccMelees:         acc.Melees,
				AccDamageDealt:    acc.DamageDealt,
				AccDamageReceived: acc.DamageReceived,
				AccCamoPickups:    acc.CamoPickups,
				AccOsPickups:      acc.OvershieldPickups,
				BestKillStreak:    acc.BestKillStreak,
			})
		}
		for _, m := range gd.Machines {
			p.Machines = append(p.Machines, GameMachine{
				Index:   m.Index,
				Name:    m.Name,
				IsLocal: m.IsLocal,
			})
		}
	}
	if c.LatestTick != nil && c.LatestTick.Network != nil {
		nw := c.LatestTick.Network
		gn := &GameNetwork{}
		if nw.Server != nil {
			gn.Countdown = &GameNetworkCountdown{
				Active: nw.Server.CountdownActive != 0,
				Paused: nw.Server.CountdownPaused != 0,
				// v1's CountdownAdjustedTime is a u8 raw value; v2 wants
				// a meaningful seconds_to_start. Until the conversion is
				// reverse-engineered, surface it as-is (cast to int16).
				SecondsToStart: int16(nw.Server.CountdownAdjustedTime),
			}
		}
		if nw.Client != nil {
			gn.Client = &GameNetworkClient{
				MachineIndex: nw.Client.MachineIndex,
				AveragePing:  nw.Client.AveragePing,
				PacketsSent:  nw.Client.PacketsSent,
			}
		}
		p.Network = gn
	}
	return p
}

// --- tick ---

// buildTickPayload returns nil when no tick has been read yet
// (cache.LatestTick == nil — runner is Idle / Ready / pre-first-tick).
// Slimmer than v1: drops extended / bones / update_queue (→ debug),
// objects / projectiles (→ objects class), network (→ game class).
func buildTickPayload(c *instanceCache) *TickPayload {
	if c.LatestTick == nil {
		return nil
	}
	src := c.LatestTick
	p := &TickPayload{
		Players:    make([]TickPlayer, 0, len(src.Players)),
		PowerItems: make([]TickPowerItem, 0, len(src.PowerItems)),
		CTFFlags:   make([]TickCTFFlag, 0, len(src.CTFFlags)),
		Locals:     make([]TickLocal, 0, len(src.Locals)),
	}
	for _, tp := range src.Players {
		p.Players = append(p.Players, convertTickPlayer(tp))
	}
	for _, pi := range src.PowerItems {
		p.PowerItems = append(p.PowerItems, TickPowerItem{
			SpawnID:        pi.SpawnID,
			Status:         pi.Status,
			HeldBy:         pi.HeldBy,
			Pos:            vec3PtrFromXYZ(pi.WorldPos),
			RespawnInTicks: pi.RespawnInTicks,
		})
	}
	for _, f := range src.CTFFlags {
		p.CTFFlags = append(p.CTFFlags, TickCTFFlag{
			Team:    f.Team,
			Pos:     vec3(f.X, f.Y, f.Z),
			Carrier: f.Carrier,
			Status:  f.Status,
		})
	}
	if src.GameGlobals != nil {
		p.GameGlobals = &TickGameGlobals{
			MapLoaded:             src.GameGlobals.MapLoaded,
			Active:                src.GameGlobals.Active,
			GameLoadingInProgress: src.GameGlobals.GameLoadingInProgress,
			PrecacheMapStatus:     src.GameGlobals.PrecacheMapStatus,
			StoredGlobalRandom:    src.GameGlobals.StoredGlobalRandom,
		}
	}
	for _, l := range src.Locals {
		p.Locals = append(p.Locals, convertTickLocal(l))
	}
	return p
}

func convertTickPlayer(tp scraper.TickPlayer) TickPlayer {
	out := TickPlayer{
		Index:              tp.Index,
		Alive:              tp.Alive,
		RespawnInTicks:     tp.RespawnInTicks,
		Pos:                vec3(tp.X, tp.Y, tp.Z),
		Vel:                vec3(tp.VX, tp.VY, tp.VZ),
		Aim:                vec3(tp.AimX, tp.AimY, tp.AimZ),
		ZoomLevel:          tp.ZoomLevel,
		CrouchScale:        tp.CrouchScale,
		Health:             tp.Health,
		Shields:            tp.Shields,
		MaxHealth:          tp.MaxHealth,
		MaxShields:         tp.MaxShields,
		HasCamo:            tp.HasCamo,
		HasOvershield:      tp.HasOvershield,
		Frags:              tp.Frags,
		Plasmas:            tp.Plasmas,
		SelectedWeaponSlot: tp.SelectedWeaponSlot,
		BipedTag:           "", // v1 reader doesn't expose the biped tag string; populated later
		Actions: TickPlayerActions{
			Crouching:       tp.IsCrouching,
			Jumping:         tp.IsJumping,
			Firing:          tp.IsFiring,
			Flashlight:      tp.IsFlashlightOn,
			ThrowingGrenade: tp.IsThrowingGrenade,
			Meleeing:        tp.IsMeleeing,
			Using:           tp.IsPressingAction || tp.IsHoldingAction,
		},
		Weapons: make([]TickWeapon, 0, len(tp.Weapons)),
	}
	for _, w := range tp.Weapons {
		tw := TickWeapon{
			Slot:     w.Slot,
			Tag:      w.Tag,
			ObjectID: w.ObjectID,
			AmmoMag:  w.AmmoMag,
			AmmoPack: w.AmmoPack,
			Charge:   w.Charge,
		}
		if w.Extended != nil {
			tw.Heat = w.Extended.HeatMeter
			tw.Reloading = w.Extended.IsReloading != 0
		}
		out.Weapons = append(out.Weapons, tw)
	}
	return out
}

func convertTickLocal(l scraper.TickLocal) TickLocal {
	out := TickLocal{LocalIndex: l.LocalIndex}
	if l.FPWeapon != nil {
		out.FPWeapon = &TickFPWeapon{
			State:         l.FPWeapon.State,
			AnimationID:   l.FPWeapon.AnimationID,
			AnimationTick: l.FPWeapon.AnimationTick,
		}
	}
	if l.ObserverCam != nil {
		out.ObserverCam = &TickObserverCam{
			Pos: vec3(l.ObserverCam.X, l.ObserverCam.Y, l.ObserverCam.Z),
			Aim: vec3(l.ObserverCam.AimX, l.ObserverCam.AimY, l.ObserverCam.AimZ),
			FOV: l.ObserverCam.FOV,
		}
	}
	if l.IAS != nil {
		out.Input = &TickLocalInput{
			LeftStick:  scraper.Vec2{X: l.IAS.LeftStickHorizontal, Y: l.IAS.LeftStickVertical},
			RightStick: scraper.Vec2{X: l.IAS.RightStickHorizontal, Y: l.IAS.RightStickVertical},
			Buttons:    TickLocalInputButtons{A: l.IAS.BtnA != 0, B: l.IAS.BtnB != 0},
		}
	}
	if l.PlayerControl != nil {
		out.PlayerControl = &TickPlayerControl{
			DesiredYaw:      l.PlayerControl.DesiredYaw,
			DesiredPitch:    l.PlayerControl.DesiredPitch,
			ZoomLevel:       l.PlayerControl.ZoomLevel,
			AimAssistTarget: optUint32(l.PlayerControl.AimAssistTarget),
		}
	}
	return out
}

// --- objects ---

// buildObjectsPayload returns nil when no tick has been read.
func buildObjectsPayload(c *instanceCache) *ObjectsPayload {
	if c.LatestTick == nil {
		return nil
	}
	src := c.LatestTick
	p := &ObjectsPayload{
		Objects:     make([]WorldObject, 0, len(src.Objects)),
		Projectiles: make([]Projectile, 0, len(src.Projectiles)),
	}
	for _, o := range src.Objects {
		p.Objects = append(p.Objects, WorldObject{
			ObjectID:       o.ObjectID,
			Tag:            o.Tag,
			Type:           o.Type,
			Flags:          o.Flags,
			Pos:            vec3(o.X, o.Y, o.Z),
			AngVel:         vec3(o.AngVelX, o.AngVelY, o.AngVelZ),
			TimeExisting:   o.TimeExisting,
			OwnerUnit:      optUint32(o.OwnerUnitRef),
			OwnerObject:    optUint32(o.OwnerObjectRef),
			UltimateParent: o.UltimateParent,
		})
	}
	for _, pj := range src.Projectiles {
		p.Projectiles = append(p.Projectiles, Projectile{
			ObjectID:         pj.ObjectID,
			Tag:              pj.Tag,
			Pos:              vec3(pj.X, pj.Y, pj.Z),
			Flags:            pj.Flags,
			Action:           pj.Action,
			DetonationTimer:  pj.DetonationTimer,
			DistanceTraveled: pj.DistanceTraveled,
		})
	}
	return p
}

// --- debug ---

// buildDebugPayload returns nil when no tick has been read. Includes the
// 66-field extended block per player, bones, and update_queue — opt-in
// per the demand model (PR 11). state_inputs / score_probe moved off this
// envelope onto the on-demand probe class (see probe.go) so probe work
// stops running per-tick.
func buildDebugPayload(c *instanceCache) *DebugPayload {
	if c.LatestTick == nil {
		return nil
	}
	src := c.LatestTick
	p := &DebugPayload{
		Players: make([]DebugPlayer, 0, len(src.Players)),
	}
	for _, tp := range src.Players {
		dp := DebugPlayer{Index: tp.Index}
		if tp.Extended != nil {
			dp.Extended = &DebugPlayerExtended{
				Legs: DebugLegRotation{
					Pitch: safeFloat(tp.Extended.LegsPitch),
					Yaw:   safeFloat(tp.Extended.LegsYaw),
					Roll:  safeFloat(tp.Extended.LegsRoll),
				},
				Airborne:      tp.Extended.Airborne != 0,
				AirborneTicks: tp.Extended.AirborneTicks,
				IdleTicks:     tp.Extended.IdleTicks,
				StateFlags:    tp.Extended.StateFlags,
				Stunned:       safeFloat(tp.Extended.Stunned),
				AimAssistSphere: &DebugSphere{
					Center: vec3(tp.Extended.AimAssistSphereX, tp.Extended.AimAssistSphereY, tp.Extended.AimAssistSphereZ),
					Radius: safeFloat(tp.Extended.AimAssistSphereRadius),
				},
			}
		}
		for _, b := range tp.Bones {
			dp.Bones = append(dp.Bones, DebugBone{
				Index: b.Index,
				Pos:   vec3(b.X, b.Y, b.Z),
			})
		}
		if tp.UpdateQueue != nil {
			dp.UpdateQueue = &DebugUpdateQueue{
				Buttons: DebugUpdateQueueButtons{
					Crouch: tp.UpdateQueue.Buttons.Crouch,
					Jump:   tp.UpdateQueue.Buttons.Jump,
					Fire:   tp.UpdateQueue.Buttons.Fire,
				},
				DesiredYaw:   safeFloat(tp.UpdateQueue.DesiredYaw),
				DesiredPitch: safeFloat(tp.UpdateQueue.DesiredPitch),
			}
		}
		p.Players = append(p.Players, dp)
	}
	return p
}

// --- previous_game ---

// buildPreviousGamePayload returns nil when no previous game has been
// captured (cache.PreviousGame == nil). Wraps the v1-stored previous
// game data in the v2 payload shape; events ride through unchanged
// (the v1 event log was already a []scraper.Envelope which itself now
// carries the v2 envelope shape from PR 4).
func buildPreviousGamePayload(c *instanceCache) *PreviousGamePayload {
	if c.PreviousGame == nil {
		return nil
	}
	prev := c.PreviousGame
	p := &PreviousGamePayload{
		EndedAt: prev.EndedAt,
		Events:  prev.Events,
	}
	// Build a synthetic GamePayload from the snapshot's GameData. The
	// previous game doesn't carry a separate snapshot's phase or
	// freshness, so those are left at the zero value.
	if prev.GameData != nil {
		shim := &instanceCache{
			GameData: prev.GameData,
		}
		gp := buildGamePayload(shim)
		p.Game = &gp
	}
	return p
}
