package manager

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// TestPushEventDualLogs: one pushEvent feeds both logs — the request_events
// ring stays newest-first, the per-match log accumulates in append
// (oldest-first) order, and neither trips the truncation flag under normal
// volume.
func TestPushEventDualLogs(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	for tick := uint32(1); tick <= 3; tick++ {
		r.pushEvent(makeEvent(tick, scraper.EventTypeDeath))
	}

	c := r.readCache()
	if len(c.Events) != 3 || c.Events[0].Tick != 3 || c.Events[2].Tick != 1 {
		t.Fatalf("ring: want newest-first [3 2 1], got %+v", tickList(c.Events))
	}
	if len(c.MatchEvents) != 3 || c.MatchEvents[0].Tick != 1 || c.MatchEvents[2].Tick != 3 {
		t.Fatalf("match log: want oldest-first [1 2 3], got %+v", tickList(c.MatchEvents))
	}
	if c.MatchEventsTruncated {
		t.Fatal("match log truncated flag set below cap")
	}
}

// TestPushEventMatchLogTruncation: past matchEventsCap the match log keeps
// its oldest-first prefix, drops the tail, and flags the truncation; the
// ring keeps rolling with the newest recentEventsCap entries regardless.
func TestPushEventMatchLogTruncation(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	total := uint32(matchEventsCap + 5)
	for tick := uint32(1); tick <= total; tick++ {
		r.pushEvent(makeEvent(tick, scraper.EventTypeDeath))
	}

	c := r.readCache()
	if len(c.MatchEvents) != matchEventsCap {
		t.Fatalf("match log len = %d, want cap %d", len(c.MatchEvents), matchEventsCap)
	}
	if !c.MatchEventsTruncated {
		t.Fatal("truncation flag not set past cap")
	}
	if c.MatchEvents[0].Tick != 1 || c.MatchEvents[matchEventsCap-1].Tick != matchEventsCap {
		t.Fatalf("match log must keep the oldest-first prefix: first=%d last=%d",
			c.MatchEvents[0].Tick, c.MatchEvents[matchEventsCap-1].Tick)
	}
	if len(c.Events) != recentEventsCap || c.Events[0].Tick != total {
		t.Fatalf("ring: want %d entries newest=%d, got %d newest=%d",
			recentEventsCap, total, len(c.Events), c.Events[0].Tick)
	}
}

// TestCaptureLiveAsPreviousMovesMatchLog: the capture moves the COMPLETE
// per-match log (not the 50-ring) into PreviousGame oldest-first, stamps
// end reason + a freshly-minted game_uid, and clears the live slots so the
// next match starts clean.
func TestCaptureLiveAsPreviousMovesMatchLog(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.GameData = &scraper.GameData{Map: "bloodgulch"}
	})
	// 60 events — more than the ring holds, so a capture that still read the
	// ring would come back with 50 newest-first entries.
	for tick := uint32(1); tick <= 60; tick++ {
		r.pushEvent(makeEvent(tick, scraper.EventTypeDeath))
	}

	r.captureLiveAsPrevious(endReasonPostgame)

	c := r.readCache()
	pg := c.PreviousGame
	if pg == nil {
		t.Fatal("capture: PreviousGame not populated")
	}
	if len(pg.Events) != 60 || pg.Events[0].Tick != 1 || pg.Events[59].Tick != 60 {
		t.Fatalf("capture: want 60 events oldest-first, got len=%d first=%d last=%d",
			len(pg.Events), pg.Events[0].Tick, pg.Events[len(pg.Events)-1].Tick)
	}
	if pg.EventsTruncated {
		t.Fatal("capture: events_truncated set without overflow")
	}
	if pg.EndReason != endReasonPostgame {
		t.Fatalf("capture: end_reason = %q, want %q", pg.EndReason, endReasonPostgame)
	}
	if pg.GameUID == "" {
		t.Fatal("capture: game_uid not minted")
	}
	if pg.EndedAt.IsZero() {
		t.Fatal("capture: ended_at not stamped")
	}
	if c.Events != nil || c.MatchEvents != nil || c.MatchEventsTruncated || c.LatestTick != nil {
		t.Fatalf("capture: live slots not cleared: events=%d match=%d truncated=%v",
			len(c.Events), len(c.MatchEvents), c.MatchEventsTruncated)
	}
}

// TestCaptureLiveAsPreviousCarriesTruncation: an overflowed match log's
// truncation flag rides into the capture (and is then reset on the cache).
func TestCaptureLiveAsPreviousCarriesTruncation(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.MatchEvents = []scraper.Envelope{makeEvent(1, scraper.EventTypeDeath)}
		c.MatchEventsTruncated = true
	})

	r.captureLiveAsPrevious(endReasonLeftMatch)

	c := r.readCache()
	if c.PreviousGame == nil || !c.PreviousGame.EventsTruncated {
		t.Fatalf("capture: truncation flag lost (prev=%+v)", c.PreviousGame)
	}
	if c.MatchEventsTruncated {
		t.Fatal("capture: cache truncation flag not reset")
	}
}

// TestCaptureLiveAsPreviousEmptyNoOp: with no game data and no events there
// is nothing to capture — PreviousGame stays nil (no phantom artifact, no
// uid minted). This is the xemu-death / releaseReader-already-ran path.
func TestCaptureLiveAsPreviousEmptyNoOp(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	r.captureLiveAsPrevious(endReasonShutdown)

	if c := r.readCache(); c.PreviousGame != nil {
		t.Fatalf("capture on empty cache: PreviousGame = %+v, want nil", c.PreviousGame)
	}
}

// TestNewGameUIDFormat: 32 lowercase hex chars, the leading 6 bytes decode
// to a unix-milliseconds timestamp near now, and consecutive mints differ.
func TestNewGameUIDFormat(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{32}$`)

	before := time.Now().UnixMilli()
	uid := newGameUID()
	after := time.Now().UnixMilli()

	if !re.MatchString(uid) {
		t.Fatalf("uid %q: want 32 lowercase hex chars", uid)
	}
	ms, err := strconv.ParseUint(uid[:12], 16, 64)
	if err != nil {
		t.Fatalf("uid %q: timestamp prefix unparsable: %v", uid, err)
	}
	if int64(ms) < before || int64(ms) > after {
		t.Fatalf("uid timestamp = %d, want within [%d, %d]", ms, before, after)
	}
	if other := newGameUID(); other == uid {
		t.Fatalf("two mints returned the same uid %q", uid)
	}
}

// TestGameUIDStableAcrossJoinReplays: the uid is minted once at capture —
// every subsequent payload build (join replay, request_state) re-serves the
// SAME value, which is what makes it usable as a consumer idempotency key.
func TestGameUIDStableAcrossJoinReplays(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.GameData = &scraper.GameData{Map: "wizard"}
	})
	r.captureLiveAsPrevious(endReasonPostgame)

	c := r.readCache()
	want := c.PreviousGame.GameUID

	for i := 0; i < 3; i++ {
		p := buildPreviousGamePayload(&c)
		if p == nil || p.GameUID != want {
			t.Fatalf("replay %d: game_uid = %+v, want stable %q", i, p, want)
		}
	}
}

// TestPreviousGamePayloadWireFields: the additive previous_game fields land
// on the wire under their contract names, and events serialize oldest-first.
func TestPreviousGamePayloadWireFields(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.GameData = &scraper.GameData{Map: "damnation"}
	})
	r.pushEvent(makeEvent(4, scraper.EventTypeDeath))
	r.pushEvent(makeEvent(9, scraper.EventTypeDeath))
	r.captureLiveAsPrevious(endReasonLeftMatch)

	c := r.readCache()
	p := buildPreviousGamePayload(&c)
	if p == nil {
		t.Fatal("buildPreviousGamePayload returned nil")
	}
	if p.EndReason != endReasonLeftMatch || p.GameUID != c.PreviousGame.GameUID || p.EventsTruncated {
		t.Fatalf("payload fields wrong: %+v", p)
	}
	if len(p.Events) != 2 || p.Events[0].Tick != 4 || p.Events[1].Tick != 9 {
		t.Fatalf("payload events: want oldest-first [4 9], got %v", tickList(p.Events))
	}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"game_uid":"` + p.GameUID + `"`, `"end_reason":"left_match"`, `"events_truncated":false`, `"ended_at":`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("wire payload missing %s in %s", want, b)
		}
	}
}

// tickList projects envelopes to their ticks for readable failure output.
func tickList(events []scraper.Envelope) []uint32 {
	out := make([]uint32, len(events))
	for i, e := range events {
		out[i] = e.Tick
	}
	return out
}
