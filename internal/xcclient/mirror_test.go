package xcclient

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/xemu-cartographer/xc-scraper/membership"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// store decodes a framed fixture and stores it, returning the result.
func store(t *testing.T, m *Mirror, raw []byte) StoreResult {
	t.Helper()
	var msg wire.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	var env wire.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatal(err)
	}
	return m.Store(env.Instance, env.Type, raw, &env)
}

func xboxFrame(t *testing.T, instance string, p wire.XboxPayload) []byte {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]any{"v": 2, "type": wire.ClassXbox, "instance": instance, "seq": 0, "tick": 0,
		"ts": "2026-09-01T12:00:00Z", "data": json.RawMessage(data)}
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return frameEnv(t, out)
}

func TestMirrorListFromXbox(t *testing.T) {
	m := NewMirror()
	started := time.Date(2026, 9, 1, 11, 42, 10, 0, time.UTC)
	m.SetInstance("smoke1", started, false)
	xb := wire.XboxPayload{
		TitleID: 0x41560003, Title: "Halo: Combat Evolved", Name: "Console-A",
		SerialNumber: "123", MACAddress: "00:50:F2:00:00:01", VideoStandard: "NTSC-M",
		TimeZone: &wire.XboxTimeZone{BiasMinutes: 480, StdName: "PST", DltName: "PDT"},
		XBE:      &wire.XboxXBE{TitleName: "Halo", Version: 1, GameRegion: 2, DiskNumber: 3, AllowedMedia: 4},
		Kernel:   &wire.XboxKernel{SystemTime: started, BootTime: started, UptimeSeconds: 1.5},
	}
	res := store(t, m, xboxFrame(t, "smoke1", xb))
	if !res.Cached || res.New {
		t.Fatalf("store result %+v", res)
	}
	store(t, m, frame(t, "game"))

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("List = %d rows", len(list))
	}
	got := list[0]
	if got.Name != "smoke1" || !got.StartedAt.Equal(started) || got.Sock != "" {
		t.Fatalf("identity fields: %+v", got)
	}
	if got.TitleID != xb.TitleID || got.Title != xb.Title || got.XboxName != "Console-A" ||
		got.SerialNumber != "123" || got.MACAddress != xb.MACAddress || got.VideoStandard != "NTSC-M" {
		t.Fatalf("xbox fields: %+v", got)
	}
	if got.TimeZoneBias != 480 || got.TimeZoneStdName != "PST" || got.TimeZoneDltName != "PDT" {
		t.Fatalf("time zone fields: %+v", got)
	}
	if got.XBETitleName != "Halo" || got.XBEVersion != 1 || got.XBEGameRegion != 2 || got.XBEDiskNumber != 3 || got.XBEAllowedMedia != 4 {
		t.Fatalf("xbe fields: %+v", got)
	}
	if got.KernelUptime != 1500*time.Millisecond || !got.KernelBootTime.Equal(started) {
		t.Fatalf("kernel fields: %+v", got)
	}
	if got.Tick != 18342 {
		t.Fatalf("Tick = %d, want the game frame's 18342", got.Tick)
	}
	st, ok := m.InstanceState("smoke1")
	if !ok || !st.Running || st.Title != xb.Title || st.XboxName != "Console-A" || st.TitleID != xb.TitleID {
		t.Fatalf("InstanceState = %+v ok=%v", st, ok)
	}
	if _, ok := m.InstanceState("nope"); ok {
		t.Fatal("unknown instance reported ok")
	}
}

func TestMirrorMembershipFromGameAndXbox(t *testing.T) {
	m := NewMirror()
	// Idle projection before any frame: just the (empty) console name.
	m.SetInstance("smoke1", time.Now(), false)
	if got := m.Membership(); len(got) != 1 || got[0].Container != "smoke1" || len(got[0].Identities) != 0 {
		t.Fatalf("idle membership = %+v", got)
	}
	store(t, m, frame(t, "game"))
	store(t, m, xboxFrame(t, "smoke1", wire.XboxPayload{Name: "Console-A"}))

	want := membership.FromGamePayload("Console-A", m.Game("smoke1"))
	got := m.Membership()
	if len(got) != 1 || !reflect.DeepEqual(got[0].Identities, want) {
		t.Fatalf("Membership = %+v, want %v", got, want)
	}
	if !membership.ContainerHasGamertag(got, "smoke1", []string{membership.SanitizeIdentity("Player1")}) {
		t.Fatal("Player1 not matched")
	}
	// A game frame after the xbox frame keeps the console name.
	store(t, m, frame(t, "game"))
	if got := m.Membership(); !reflect.DeepEqual(got[0].Identities, want) {
		t.Fatalf("after game refresh: %v", got[0].Identities)
	}
}

func TestMirrorJoinReplayOrder(t *testing.T) {
	m := NewMirror()
	scenario1 := frame(t, "scenario")
	game1 := frame(t, "game")
	prev1 := frame(t, "previous_game")
	tick1 := frame(t, "tick")
	event1 := frame(t, "event_death")
	game2 := frameEnv(t, mutated(t, "game", map[string]any{"instance": "smoke2"}))
	summary := frame(t, "summary")

	for _, raw := range [][]byte{tick1, game2, prev1, event1, game1, scenario1} {
		store(t, m, raw)
	}
	var sp wire.SummaryPayload
	m.StoreSummary(summary, &sp)

	if got := m.Instances(); !reflect.DeepEqual(got, []string{"smoke1", "smoke2"}) {
		t.Fatalf("Instances = %v", got)
	}
	// Instances order × StateClasses order (xbox scenario game game_filtered
	// previous_game tick objects debug); events never replay.
	want := [][]byte{scenario1, game1, prev1, tick1, game2}
	got := m.JoinReplayMessages()
	if len(got) != len(want) {
		t.Fatalf("JoinReplayMessages = %d frames, want %d", len(got), len(want))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Fatalf("frame %d differs:\n got %s\nwant %s", i, got[i], want[i])
		}
	}
	if got := m.JoinReplayForInstance("smoke1"); len(got) != 4 || !bytes.Equal(got[0], scenario1) || !bytes.Equal(got[3], tick1) {
		t.Fatalf("JoinReplayForInstance = %d frames", len(got))
	}
	if got := m.JoinReplayForInstance("nope"); got != nil {
		t.Fatalf("unknown instance replay = %v", got)
	}
	if got := m.JoinReplayForInstanceClass("smoke1", wire.ClassGame); len(got) != 1 || !bytes.Equal(got[0], game1) {
		t.Fatalf("JoinReplayForInstanceClass = %v", got)
	}
	if got := m.JoinReplayForInstanceClass("smoke1", wire.ClassEvent); got != nil {
		t.Fatalf("event class replay = %v", got)
	}
	if got := m.JoinReplayForHostAll(); len(got) != 1 || !bytes.Equal(got[0], summary) {
		t.Fatalf("JoinReplayForHostAll = %v", got)
	}
	m.Drop("smoke1", wire.ClassTick)
	if got := m.JoinReplayForInstance("smoke1"); len(got) != 3 {
		t.Fatalf("after Drop: %d frames", len(got))
	}
	if m.Frame("smoke1", wire.ClassGame) == nil || m.Frame("smoke1", wire.ClassTick) != nil {
		t.Fatal("Frame lookup after Drop")
	}
}

func TestMirrorSetInstanceEpoch(t *testing.T) {
	m := NewMirror()
	ts := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	real := ts.Add(-time.Hour)
	if m.SetInstance("a", ts, true) {
		t.Fatal("new placeholder instance counted as epoch change")
	}
	store(t, m, frameEnv(t, mutated(t, "game", map[string]any{"instance": "a"})))
	if got := m.Placeholders(); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("Placeholders = %v", got)
	}
	if m.SetInstance("a", real, false) {
		t.Fatal("placeholder → real counted as epoch change")
	}
	if at, _ := m.StartedAt("a"); !at.Equal(real) || len(m.Placeholders()) != 0 {
		t.Fatalf("started_at after refresh = %v", at)
	}
	if m.Game("a") == nil {
		t.Fatal("placeholder refresh dropped the cache")
	}
	if m.SetInstance("a", real, false) {
		t.Fatal("same started_at counted as epoch change")
	}
	if m.SetInstance("a", real, true) {
		t.Fatal("placeholder over a real value counted as epoch change")
	}
	if at, _ := m.StartedAt("a"); !at.Equal(real) {
		t.Fatal("placeholder overwrote a real started_at")
	}
	if !m.SetInstance("a", real.Add(time.Minute), false) {
		t.Fatal("changed started_at not reported")
	}
	if m.Game("a") != nil || m.JoinReplayForInstance("a") != nil {
		t.Fatal("epoch change kept the cache")
	}
	if !m.Remove("a") || m.Remove("a") {
		t.Fatal("Remove")
	}
}

func TestMirrorSeqTracking(t *testing.T) {
	m := NewMirror()
	game := func(seq uint64) []byte {
		return frameEnv(t, mutated(t, "game", map[string]any{"seq": seq}))
	}
	res := store(t, m, game(41))
	if !res.New || res.Regression || res.Gap != 0 {
		t.Fatalf("first frame: %+v", res)
	}
	if at, ok := m.StartedAt("smoke1"); !ok || !at.Equal(time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("placeholder started_at = %v ok=%v", at, ok)
	}
	if res := store(t, m, game(41)); res.New || res.Regression || res.Gap != 0 {
		t.Fatalf("replayed frame: %+v", res)
	}
	if res := store(t, m, game(45)); res.Gap != 3 || res.Regression {
		t.Fatalf("gap frame: %+v", res)
	}
	store(t, m, frame(t, "scenario"))
	if res := store(t, m, game(2)); !res.Regression || res.New {
		t.Fatalf("regression frame: %+v", res)
	}
	if m.Frame("smoke1", wire.ClassScenario) != nil {
		t.Fatal("regression did not clear the other classes")
	}
	if m.Frame("smoke1", wire.ClassGame) == nil {
		t.Fatal("regression dropped the new frame")
	}
	for i := 0; i < 2; i++ {
		if res := store(t, m, frame(t, "event_death")); res.Cached || res.Regression || res.Gap != 0 {
			t.Fatalf("event frame %d cached or seq-tracked: %+v", i, res)
		}
	}
	if res := m.Store("smoke1", wire.ClassEvents, []byte("{}"), &wire.Envelope{}); res.Cached {
		t.Fatal("events reply cached")
	}
	m.ClearInstance("smoke1")
	if m.Frame("smoke1", wire.ClassGame) != nil || len(m.Instances()) != 1 {
		t.Fatal("ClearInstance")
	}
	m.Clear()
	if len(m.Instances()) != 0 || m.JoinReplayForHostAll() != nil {
		t.Fatal("Clear")
	}
}
