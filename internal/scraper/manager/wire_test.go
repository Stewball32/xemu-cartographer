package manager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
)

// newTestRunner builds a minimal *runner suitable for exercising the
// envelope builders without standing up a real xemu instance or aggregator.
// Tests prepare the cache directly under r.cacheMu (or via withCache), then
// call buildCurrentStateEnvelope / buildStateUpdateEnvelope.
func newTestRunner(name string) *runner {
	return newRunner(name, "/tmp/sock", "host:"+name, nil, nil)
}

// decodeEnvelope unwraps a marshaled websocket.Message to its inner
// scraper.Envelope so tests can assert on Type / Instance / Tick / Payload.
func decodeEnvelope(t *testing.T, data []byte) (websocket.Message, scraper.Envelope) {
	t.Helper()
	var msg websocket.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal websocket.Message: %v", err)
	}
	var env scraper.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	return msg, env
}

// TestBuildCurrentStateEnvelopeIdle covers the bare cache shape that exists
// before any reader has been bound. Phase + identity + freshness are
// present; per-match data is absent.
func TestBuildCurrentStateEnvelopeIdle(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	now := time.Now()
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseIdle
		c.TitleID = 0x9E115330
		c.LastReadAt = now
		c.Iterations = 5
	})

	bytes, ok := r.buildCurrentStateEnvelope()
	if !ok {
		t.Fatal("buildCurrentStateEnvelope: ok=false")
	}
	msg, env := decodeEnvelope(t, bytes)

	if msg.Type != "scraper" {
		t.Fatalf("websocket.Message.Type = %q, want %q", msg.Type, "scraper")
	}
	if msg.Room != "host:alpha" {
		t.Fatalf("websocket.Message.Room = %q, want %q", msg.Room, "host:alpha")
	}
	if env.Type != envelopeTypeCurrentState {
		t.Fatalf("envelope type = %q, want %q", env.Type, envelopeTypeCurrentState)
	}
	if env.Instance != "alpha" {
		t.Fatalf("envelope instance = %q, want %q", env.Instance, "alpha")
	}
	if env.Tick != 0 {
		t.Fatalf("envelope tick = %d, want 0 (Idle)", env.Tick)
	}

	var p CurrentStatePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		t.Fatalf("unmarshal CurrentStatePayload: %v", err)
	}
	if p.Phase != PhaseIdle {
		t.Fatalf("payload phase = %q, want %q", p.Phase, PhaseIdle)
	}
	if p.TitleID != 0x9E115330 {
		t.Fatalf("payload title_id = %#x, want 0x9E115330", p.TitleID)
	}
	if p.Iterations != 5 {
		t.Fatalf("payload iterations = %d, want 5", p.Iterations)
	}
	if p.GameData != nil {
		t.Fatalf("payload game_data = %+v, want nil in Idle", p.GameData)
	}
	if p.LatestTick != nil {
		t.Fatalf("payload latest_tick = %+v, want nil in Idle", p.LatestTick)
	}
	if p.PreviousGame != nil {
		t.Fatalf("payload previous_game = %+v, want nil in Idle", p.PreviousGame)
	}
}

// TestBuildCurrentStateEnvelopeLive covers the full-cache shape during an
// in-progress match: GameData, LatestTick, and the engine tick all surface
// on the envelope.
func TestBuildCurrentStateEnvelopeLive(t *testing.T) {
	r := newTestRunner("bravo")
	defer r.cancel()

	gd := &scraper.GameData{Map: "bloodgulch", Gametype: "slayer"}
	tp := &scraper.TickPayload{PlayerCount: 4}
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.TitleID = 0x4D530004
		c.Title = "Halo: Combat Evolved"
		c.EngineTick = 1234
		c.GameData = gd
		c.LatestTick = tp
	})

	bytes, ok := r.buildCurrentStateEnvelope()
	if !ok {
		t.Fatal("buildCurrentStateEnvelope: ok=false")
	}
	_, env := decodeEnvelope(t, bytes)

	if env.Tick != 1234 {
		t.Fatalf("envelope tick = %d, want 1234", env.Tick)
	}

	var p CurrentStatePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		t.Fatalf("unmarshal CurrentStatePayload: %v", err)
	}
	if p.Phase != PhaseLive {
		t.Fatalf("payload phase = %q, want %q", p.Phase, PhaseLive)
	}
	if p.GameData == nil || p.GameData.Map != "bloodgulch" {
		t.Fatalf("payload game_data = %+v, want bloodgulch slayer", p.GameData)
	}
	if p.LatestTick == nil || p.LatestTick.PlayerCount != 4 {
		t.Fatalf("payload latest_tick = %+v, want PlayerCount=4", p.LatestTick)
	}
	if p.EngineTick != 1234 {
		t.Fatalf("payload engine_tick = %d, want 1234", p.EngineTick)
	}
}

// TestBuildCurrentStateEnvelopeReadyWithPreviousGame covers the postgame
// case: PreviousGame slot populated after a Live → Ready transition.
func TestBuildCurrentStateEnvelopeReadyWithPreviousGame(t *testing.T) {
	r := newTestRunner("charlie")
	defer r.cancel()

	prev := &previousGame{
		GameData: &scraper.GameData{Map: "wizard"},
		EndedAt:  time.Now(),
	}
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseReady
		c.GameData = &scraper.GameData{Map: "wizard"}
		c.PreviousGame = prev
	})

	bytes, ok := r.buildCurrentStateEnvelope()
	if !ok {
		t.Fatal("buildCurrentStateEnvelope: ok=false")
	}
	_, env := decodeEnvelope(t, bytes)

	var p CurrentStatePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		t.Fatalf("unmarshal CurrentStatePayload: %v", err)
	}
	if p.PreviousGame == nil {
		t.Fatal("payload previous_game = nil, want populated in Ready postgame")
	}
	if p.PreviousGame.GameData == nil || p.PreviousGame.GameData.Map != "wizard" {
		t.Fatalf("payload previous_game.game_data = %+v, want wizard", p.PreviousGame.GameData)
	}
}

// TestBuildStateUpdateEnvelopeIdle: Idle state_update carries no per-phase
// payload; envelope tick is 0.
func TestBuildStateUpdateEnvelopeIdle(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseIdle
		c.Iterations = 7
		c.EngineTick = 999 // should not leak to envelope tick outside Live
	})

	bytes, ok := r.buildStateUpdateEnvelope(PhaseIdle)
	if !ok {
		t.Fatal("buildStateUpdateEnvelope: ok=false")
	}
	_, env := decodeEnvelope(t, bytes)

	if env.Type != envelopeTypeStateUpdate {
		t.Fatalf("envelope type = %q, want %q", env.Type, envelopeTypeStateUpdate)
	}
	if env.Tick != 0 {
		t.Fatalf("envelope tick = %d, want 0 outside Live", env.Tick)
	}

	var p StateUpdatePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		t.Fatalf("unmarshal StateUpdatePayload: %v", err)
	}
	if p.Phase != PhaseIdle {
		t.Fatalf("payload phase = %q, want %q", p.Phase, PhaseIdle)
	}
	if p.Iterations != 7 {
		t.Fatalf("payload iterations = %d, want 7", p.Iterations)
	}
	if p.Tick != nil {
		t.Fatalf("payload tick = %+v, want nil outside Live", p.Tick)
	}
	if p.Ready != nil {
		t.Fatalf("payload ready = %+v, want nil outside Ready", p.Ready)
	}
	if p.EngineTick != 0 {
		t.Fatalf("payload engine_tick = %d, want 0 outside Live", p.EngineTick)
	}
}

// TestBuildStateUpdateEnvelopeReady: Ready state_update carries the cache's
// GameData under .Ready; envelope tick remains 0.
func TestBuildStateUpdateEnvelopeReady(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	gd := &scraper.GameData{Map: "sidewinder", Gametype: "ctf"}
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseReady
		c.GameData = gd
	})

	bytes, ok := r.buildStateUpdateEnvelope(PhaseReady)
	if !ok {
		t.Fatal("buildStateUpdateEnvelope: ok=false")
	}
	_, env := decodeEnvelope(t, bytes)

	if env.Tick != 0 {
		t.Fatalf("envelope tick = %d, want 0 in Ready", env.Tick)
	}

	var p StateUpdatePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		t.Fatalf("unmarshal StateUpdatePayload: %v", err)
	}
	if p.Phase != PhaseReady {
		t.Fatalf("payload phase = %q, want %q", p.Phase, PhaseReady)
	}
	if p.Ready == nil || p.Ready.Map != "sidewinder" {
		t.Fatalf("payload ready = %+v, want sidewinder ctf", p.Ready)
	}
	if p.Tick != nil {
		t.Fatalf("payload tick = %+v, want nil in Ready", p.Tick)
	}
}

// TestBuildStateUpdateEnvelopeLive: Live state_update carries the cache's
// LatestTick + engine tick on the top-level envelope.
func TestBuildStateUpdateEnvelopeLive(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	tp := &scraper.TickPayload{PlayerCount: 2}
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.EngineTick = 4242
		c.LatestTick = tp
	})

	bytes, ok := r.buildStateUpdateEnvelope(PhaseLive)
	if !ok {
		t.Fatal("buildStateUpdateEnvelope: ok=false")
	}
	_, env := decodeEnvelope(t, bytes)

	if env.Tick != 4242 {
		t.Fatalf("envelope tick = %d, want 4242 (engine tick) in Live", env.Tick)
	}

	var p StateUpdatePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		t.Fatalf("unmarshal StateUpdatePayload: %v", err)
	}
	if p.Phase != PhaseLive {
		t.Fatalf("payload phase = %q, want %q", p.Phase, PhaseLive)
	}
	if p.EngineTick != 4242 {
		t.Fatalf("payload engine_tick = %d, want 4242", p.EngineTick)
	}
	if p.Tick == nil || p.Tick.PlayerCount != 2 {
		t.Fatalf("payload tick = %+v, want PlayerCount=2", p.Tick)
	}
	if p.Ready != nil {
		t.Fatalf("payload ready = %+v, want nil in Live", p.Ready)
	}
}
