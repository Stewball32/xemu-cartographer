package xcclient

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/xemu-cartographer/xc-scraper/wire"
)

// FinishedGameGrace is how long after a previous_game frame names a
// finished game the league waits for its games row before logging the D-7
// line (DESIGN-STEP8 §7.2: webhook misconfiguration detector).
const FinishedGameGrace = 30 * time.Second

// FinishedGameDetector is the D-7 webhook-misconfiguration detector: every
// previous_game frame whose finished_game carries a game_uid arms a timer;
// when it fires and Exists(uid) is still false the detector logs
// "xcclient: finished_game <uid> seen on stream, not persisted after 30s".
// A wrong or expired --webhook-token, a league URL behind a redirect or a
// daemon without --game-webhook all leave games unpersisted while the
// stream stays perfectly healthy; this is the league-side signal for it.
// Each uid is checked once per process (replays of the same previous_game
// on every join would otherwise re-arm it). Compose OnFrame into
// Config.OnFrame; PB-free — the games lookup is injected.
type FinishedGameDetector struct {
	exists func(uid string) bool
	logf   func(string, ...any)
	grace  time.Duration

	mu    sync.Mutex
	seen  map[string]struct{}
	timer func(time.Duration, func()) // time.AfterFunc; swapped in tests
}

// NewFinishedGameDetector returns a detector over exists (true when a games
// row with that game_uid is persisted). A nil exists or logf disables it.
func NewFinishedGameDetector(exists func(uid string) bool, logf func(string, ...any)) *FinishedGameDetector {
	return &FinishedGameDetector{
		exists: exists,
		logf:   logf,
		grace:  FinishedGameGrace,
		seen:   make(map[string]struct{}),
		timer:  func(d time.Duration, fn func()) { time.AfterFunc(d, fn) },
	}
}

// OnFrame inspects previous_game frames (Config.OnFrame shape).
func (d *FinishedGameDetector) OnFrame(f Frame) {
	if d == nil || d.exists == nil || d.logf == nil || f.Env == nil || f.Env.Type != wire.ClassPreviousGame {
		return
	}
	var p struct {
		FinishedGame *struct {
			GameUID string `json:"game_uid"`
		} `json:"finished_game"`
	}
	if json.Unmarshal(f.Env.Data, &p) != nil || p.FinishedGame == nil || p.FinishedGame.GameUID == "" {
		return
	}
	uid := p.FinishedGame.GameUID
	d.mu.Lock()
	if _, dup := d.seen[uid]; dup {
		d.mu.Unlock()
		return
	}
	d.seen[uid] = struct{}{}
	d.mu.Unlock()
	d.timer(d.grace, func() {
		if d.exists(uid) {
			return
		}
		d.logf("xcclient: finished_game %s seen on stream, not persisted after %s", uid, d.grace)
	})
}

// Seen reports how many distinct finished-game uids the detector has armed.
func (d *FinishedGameDetector) Seen() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.seen)
}
