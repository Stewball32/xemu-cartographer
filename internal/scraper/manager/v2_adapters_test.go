package manager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// TestAdaptersOnEmptyCache: each adapter handles a freshly-constructed
// instanceCache without panicking. State-class adapters that require
// data return nil; XboxPayload and GamePayload always return a value
// (they have non-data fields like Phase / Title).
func TestAdaptersOnEmptyCache(t *testing.T) {
	c := &instanceCache{Phase: PhaseIdle}

	xbox := buildXboxPayload(c)
	if xbox.TimeZone != nil || xbox.XBE != nil || xbox.Kernel != nil {
		t.Fatalf("empty cache: xbox optionals should be nil, got %+v", xbox)
	}

	if buildScenarioPayload(c) != nil {
		t.Fatal("empty cache: scenario should be nil")
	}
	if buildTickPayload(c) != nil {
		t.Fatal("empty cache: tick should be nil")
	}
	if buildObjectsPayload(c) != nil {
		t.Fatal("empty cache: objects should be nil")
	}
	if buildDebugPayload(c) != nil {
		t.Fatal("empty cache: debug should be nil")
	}
	if buildPreviousGamePayload(c) != nil {
		t.Fatal("empty cache: previous_game should be nil")
	}

	game := buildGamePayload(c)
	if game.Phase != PhaseIdle {
		t.Fatalf("game.Phase = %q, want %q", game.Phase, PhaseIdle)
	}
	if game.Config != nil {
		t.Fatalf("idle game: Config should be nil, got %+v", game.Config)
	}
}

// TestXboxAdapterPopulatesNestedObjects: TimeZone / XBE / Kernel only
// become non-nil when their gate fields (StdName / TitleName /
// !SystemTime.IsZero()) are set. Zero-valued numerics (bias=0,
// disk_number=0) survive inside the nested objects — the v1 omitempty
// bug fix.
func TestXboxAdapterPopulatesNestedObjects(t *testing.T) {
	c := &instanceCache{
		TitleID:          0x4D530102,
		Title:            "Halo: Combat Evolved",
		XboxName:         "Stewart's Xbox",
		SerialNumber:     "712998856516",
		MACAddress:       "00:50:f2:96:b6:6f",
		VideoStandard:    "NTSC-M",
		TimeZoneBias:     0, // GMT — the zero-valid case
		TimeZoneStdName:  "GMT",
		TimeZoneDltName:  "BST",
		XBETitleName:     "Halo (Server)",
		XBEVersion:       8,
		XBEGameRegion:    1,
		XBEDiskNumber:    0, // also zero-valid
		XBEAllowedMedia:  127,
		KernelSystemTime: time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC),
		KernelBootTime:   time.Date(2026, 5, 16, 9, 30, 0, 0, time.UTC),
		KernelUptime:     30 * time.Minute,
	}
	p := buildXboxPayload(c)
	if p.TimeZone == nil || p.TimeZone.BiasMinutes != 0 || p.TimeZone.StdName != "GMT" {
		t.Fatalf("TimeZone = %+v, want bias=0 std=GMT", p.TimeZone)
	}
	if p.XBE == nil || p.XBE.DiskNumber != 0 || p.XBE.AllowedMedia != 127 {
		t.Fatalf("XBE = %+v, want disk=0 media=127", p.XBE)
	}
	if p.Kernel == nil || p.Kernel.UptimeSeconds != 1800 {
		t.Fatalf("Kernel = %+v, want uptime_seconds=1800", p.Kernel)
	}

	// Marshaled wire shape: bias_minutes=0 must be present (the v1 bug).
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal xbox: %v", err)
	}
	want := []string{`"bias_minutes":0`, `"disk_number":0`, `"uptime_seconds":1800`}
	for _, sub := range want {
		if !contains(string(b), sub) {
			t.Fatalf("xbox JSON missing %q: %s", sub, b)
		}
	}
}

// TestTickAdapterConvertsPlayer: per-player conversion produces the
// v2 nested pos/vel/aim Vec3s, the collapsed actions object, the
// trimmed weapon shape, and the cleared inline biped_tag string.
func TestTickAdapterConvertsPlayer(t *testing.T) {
	mag := int16(60)
	pack := int16(180)
	tickReadout := &scraper.TickPayload{
		Players: []scraper.TickPlayer{{
			Index: 0,
			Alive: true,
			X:     1.5, Y: 2.5, Z: 3.5,
			VX: 0.1, VY: 0.2, VZ: 0.3,
			AimX: 1, AimY: 0, AimZ: 0,
			ZoomLevel:          -1,
			Health:             1,
			Shields:            1,
			Frags:              4,
			SelectedWeaponSlot: 0,
			IsCrouching:        true,
			IsFiring:           true,
			IsPressingAction:   true,
			Weapons: []scraper.WeaponInfo{{
				Slot:     0,
				Tag:      "weapons\\pistol\\pistol",
				ObjectID: 42,
				AmmoMag:  &mag,
				AmmoPack: &pack,
				Extended: &scraper.TickWeaponObjectExtended{
					HeatMeter:   0.25,
					IsReloading: 1,
				},
			}},
		}},
	}
	c := &instanceCache{LatestTick: tickReadout}

	tp := buildTickPayload(c)
	if tp == nil || len(tp.Players) != 1 {
		t.Fatalf("tick payload missing or wrong player count: %+v", tp)
	}
	p := tp.Players[0]
	if p.Pos.X != 1.5 || p.Pos.Y != 2.5 || p.Pos.Z != 3.5 {
		t.Fatalf("pos = %+v, want {1.5,2.5,3.5}", p.Pos)
	}
	if p.Vel.Z != 0.3 || p.Aim.X != 1 {
		t.Fatalf("vel/aim wrong: vel=%+v aim=%+v", p.Vel, p.Aim)
	}
	if !p.Actions.Crouching || !p.Actions.Firing || !p.Actions.Using {
		t.Fatalf("actions wrong: %+v", p.Actions)
	}
	if p.Actions.Jumping || p.Actions.Meleeing {
		t.Fatalf("actions wrong (false expected): %+v", p.Actions)
	}
	if p.BipedTag != "" {
		t.Fatalf("biped_tag = %q, want empty (v1 reader doesn't expose name yet)", p.BipedTag)
	}
	if len(p.Weapons) != 1 {
		t.Fatalf("weapons len = %d, want 1", len(p.Weapons))
	}
	w := p.Weapons[0]
	if w.Tag != "weapons\\pistol\\pistol" || w.ObjectID != 42 {
		t.Fatalf("weapon tag/id wrong: %+v", w)
	}
	if w.AmmoMag == nil || *w.AmmoMag != 60 {
		t.Fatalf("ammo_mag wrong: %v", w.AmmoMag)
	}
	if w.Heat != 0.25 || !w.Reloading {
		t.Fatalf("weapon heat/reloading wrong: heat=%v reloading=%v", w.Heat, w.Reloading)
	}
}

// TestObjectsAdapterPromotesOwnerSentinels: 0xFFFFFFFF (no-owner
// sentinel) becomes nil; real owner refs become pointers.
func TestObjectsAdapterPromotesOwnerSentinels(t *testing.T) {
	c := &instanceCache{LatestTick: &scraper.TickPayload{
		Objects: []scraper.TickObject{
			{ObjectID: 1, Tag: "vehicles\\warthog\\warthog", X: 1, Y: 2, Z: 3,
				OwnerUnitRef: 0xFFFFFFFF, OwnerObjectRef: 0xFFFFFFFF},
			{ObjectID: 2, Tag: "weapons\\frag grenade\\frag grenade", X: 4, Y: 5, Z: 6,
				OwnerUnitRef: 100, OwnerObjectRef: 200},
		},
	}}
	op := buildObjectsPayload(c)
	if op == nil || len(op.Objects) != 2 {
		t.Fatalf("objects payload wrong: %+v", op)
	}
	if op.Objects[0].OwnerUnit != nil || op.Objects[0].OwnerObject != nil {
		t.Fatalf("sentinel owner refs should be nil, got unit=%v object=%v", op.Objects[0].OwnerUnit, op.Objects[0].OwnerObject)
	}
	if op.Objects[1].OwnerUnit == nil || *op.Objects[1].OwnerUnit != 100 {
		t.Fatalf("real owner_unit wrong: %v", op.Objects[1].OwnerUnit)
	}
}

// TestGameAdapterNetworkFromTick: GameNetwork is built from the
// LatestTick.Network (which is where the v1 reader places it), not
// from GameData. v2 design moves network onto the game class because
// it's slow-changing and doesn't need 30 Hz cadence.
func TestGameAdapterNetworkFromTick(t *testing.T) {
	c := &instanceCache{
		Phase: PhaseLive,
		GameData: &scraper.GameData{
			Map:      "bloodgulch",
			Gametype: "slayer",
		},
		LatestTick: &scraper.TickPayload{
			Network: &scraper.TickNetwork{
				Server: &scraper.TickNetworkServer{CountdownActive: 1},
				Client: &scraper.TickNetworkClient{MachineIndex: 0, AveragePing: 42},
			},
		},
	}
	gp := buildGamePayload(c)
	if gp.Config == nil || gp.Config.Gametype != "slayer" {
		t.Fatalf("config = %+v, want slayer", gp.Config)
	}
	if gp.Network == nil || gp.Network.Client == nil || gp.Network.Client.AveragePing != 42 {
		t.Fatalf("network = %+v, want ping=42", gp.Network)
	}
	if gp.Network.Countdown == nil || !gp.Network.Countdown.Active {
		t.Fatalf("countdown = %+v, want active=true", gp.Network.Countdown)
	}
}

// contains is a tiny strings.Contains substitute to avoid the import
// just for one use.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
