package manager

import (
	"context"
	"fmt"
	"log"
	"time"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/customvariants"
)

// hostTickMinInterval bounds how often the host runner is ticked. 100ms
// (menu-nav pacing pass 2026-08-10): with the screen-record classifier the
// per-tick cost is 2 fixed reads + cached resolves on top of the reads the
// Ready loop already does, and the runner's tightened closed-loop timings
// (NavKeyInterval 150ms, RepressAfter 350ms) need observations at this cadence
// to confirm presses promptly — at the old 400ms a press's confirmation
// (measured +31…176ms) always burned a whole extra tick. The Ready loop's
// 500ms iteration stays the cadence for broadcasts/refreshes; hostSubTicks
// slices its sleep to re-read state + tick the runner at this interval, and
// ONLY while a host runner is attached (no cost for plain scraped boxes).
const hostTickMinInterval = 100 * time.Millisecond

// enumMinInterval bounds how often the create-game carousel enumeration runs.
// The lists change only when a disc / gametype set changes, so a few-second
// spacing is ample and keeps the ~1000-entry tag-array walk off the hot path.
const enumMinInterval = 3 * time.Second

// enumerateLobby refreshes the runner's available-maps/gametypes cache from the
// live create-game carousels, feeding the same seam Manager.SetAvailableMaps
// writes (so /api/play/options serves real per-instance lists). No-op when the
// bound GameReader can't enumerate (title without a LobbyEnumerator) or the tick
// throttle hasn't elapsed. On an unavailable read (mid-match, tags not loaded) it
// KEEPS the last successful enumeration rather than clobbering it with empties.
// Loop-goroutine only (reads r.reader directly).
func (r *runner) enumerateLobby() {
	enum, ok := r.reader.(scraper.LobbyEnumerator)
	if !ok {
		return
	}
	now := time.Now()
	if now.Sub(r.lastEnumAt) < enumMinInterval {
		return
	}
	r.lastEnumAt = now

	opts := enum.EnumerateLobby()
	if !opts.Available {
		return
	}
	maps := toMapOptions(opts.Maps)
	builtinGametypes := toMapOptions(opts.Gametypes)
	// Kick the one-time host-side read of the box's saved custom variants (part C);
	// no-op after the first call. Results land in cache.CustomGametypes and get
	// PREPENDED below on subsequent ticks, so the served list == the live carousel.
	r.ensureCustomVariants()
	changed := false
	var gtCount int
	r.withCache(func(c *instanceCache) {
		gametypes := prependCustomGametypes(c.CustomGametypes, builtinGametypes)
		changed = len(c.AvailableMaps) != len(maps) || len(c.AvailableGametypes) != len(gametypes)
		c.AvailableMaps = maps
		c.AvailableGametypes = gametypes
		gtCount = len(gametypes)
	})
	// Log on count-change so beta.log carries the ground truth for the /play
	// picker (grep: "lobby enumerated"). Crucially surfaces the split case — real
	// maps but empty gametypes (e.g. a build whose gametype-names tag differs) —
	// which otherwise looks like a silent frontend bug. On-change only: the lists
	// are stable per disc, so this is one line per box per change, not per 3s tick.
	// The gametype count includes any prepended custom variants once they load.
	if changed {
		log.Printf("scraper[%s]: lobby enumerated: %d maps, %d gametypes (available=%v)",
			r.name, len(maps), gtCount, opts.Available)
	}
}

// ensureCustomVariants fires the one-time, async host-side read of this box's
// saved custom gametype variants (customvariants → the overlay's UDATA), off the
// hot loop. No-op when the overlay resolver isn't wired, and self-limiting via
// customOnce so a failed / empty read isn't retried every tick. Loop-goroutine
// only (customOnce.Do), but the read itself runs on its own goroutine and stores
// into cache.CustomGametypes under cacheMu.
func (r *runner) ensureCustomVariants() {
	if r.overlayFor == nil {
		return
	}
	r.customOnce.Do(func() {
		name := r.name
		title := fmt.Sprintf("%08x", r.cachedTitleID())
		go func() {
			// Mark the load DONE on every exit (success, empty, or error) so the
			// play API stops showing "reading gametypes…" and reveals the list.
			defer r.withCache(func(c *instanceCache) { c.CustomLoadDone = true })
			path, ok := r.overlayFor(name)
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(r.ctx, 45*time.Second)
			defer cancel()
			names, err := customvariants.Names(ctx, path, title)
			if err != nil {
				log.Printf("scraper[%s]: custom gametype variants unavailable (built-ins only): %v", name, err)
				return
			}
			if len(names) == 0 {
				return
			}
			r.withCache(func(c *instanceCache) {
				c.CustomGametypes = names
				// Merge into the SERVED list right now, in the same critical section —
				// don't wait for the next enumerateLobby tick. CustomLoadDone (the
				// gametypes_pending signal) flips the moment this goroutine returns, so
				// if the merge lagged a tick the play API would report "list complete"
				// while AvailableGametypes was still built-ins-only; the frontend then
				// stopped re-polling and cached built-ins until a HARD REFRESH (the
				// reported bug). AvailableGametypes is built-ins-only here (customs were
				// empty until this assignment) so this prepend can't double-apply.
				if len(c.AvailableGametypes) > 0 {
					c.AvailableGametypes = prependCustomGametypes(names, c.AvailableGametypes)
				}
			})
			log.Printf("scraper[%s]: %d custom gametype variants loaded from disk", name, len(names))
		}()
	})
}

// prependCustomGametypes puts the box's user-saved custom variants (already in
// carousel order) AHEAD of the built-in gametypes and reassigns every option's
// Steps to its ABSOLUTE carousel index. This makes the served list == the live
// SELECT GAMETYPE carousel 1:1, and a pick's Steps is its widget card index
// directly: the runner's gametypeCustomPrefix then computes 0 (live count ==
// enumerated count), so both custom and built-in picks navigate to the right
// card. Empty customs → built-ins pass through unchanged (Steps already 0..n-1).
func prependCustomGametypes(customNames []string, builtins []scraperiface.MapOption) []scraperiface.MapOption {
	if len(customNames) == 0 {
		return builtins
	}
	out := make([]scraperiface.MapOption, 0, len(customNames)+len(builtins))
	for _, nm := range customNames {
		out = append(out, scraperiface.MapOption{Name: nm})
	}
	out = append(out, builtins...)
	for i := range out {
		out[i].Steps = i
	}
	return out
}

// toMapOptions maps the game-agnostic scraper.LobbyOption slice onto the
// scraperiface.MapOption the player API serves (identical shape; kept separate so
// game plugins carry no guards-package dependency).
func toMapOptions(in []scraper.LobbyOption) []scraperiface.MapOption {
	out := make([]scraperiface.MapOption, len(in))
	for i, o := range in {
		out[i] = scraperiface.MapOption{Name: o.Name, Steps: o.Steps}
	}
	return out
}

// tickHost drives one step of the player-hosting runner from the scraper loop
// goroutine. It is a no-op when host-running is disabled or the tick throttle
// hasn't elapsed. The Observation is built from LOOP-OWNED state only — the
// reader's LastStateInputs (main_menu / game_connection) and the loop's working
// GameData copy — so it never touches a GameReader from another goroutine (the
// runner's key presses go out asynchronously via the vncinput pump). Called with
// the freshly-read game state + tick from runReady / runLive.
func (r *runner) tickHost(gs scraper.GameState, tick uint32) {
	now := time.Now()
	if now.Sub(r.lastHostTickAt) < hostTickMinInterval {
		return
	}
	r.lastHostTickAt = now
	// Build + PUBLISH the readout on EVERY tick, whether or not a host runner is
	// attached. The admin diagnostics panel reads this; gating it on r.host was what
	// froze the panel at tick 0 on any box with host-running disabled (the navfp log
	// kept updating because it comes from the reader, not the runner).
	ro := r.buildHostReadout(gs, tick)
	// Liveness stamps: Seq advances every tick so the panel can prove it is live even
	// at the menus, where the GAME tick is legitimately 0.
	r.readoutSeq++
	ro.Seq = r.readoutSeq
	ro.ReadAtUnixMs = now.UnixMilli()
	r.setReadout(ro)
	if r.host == nil {
		return
	}
	r.host.Tick(ro.Observation(), now)
}

// hostSubTicks sleeps out the remainder of a Ready iteration in
// hostTickMinInterval slices, re-reading game state and ticking the host runner
// between slices — the fast path that gives the runner ~100ms observations
// while the Ready loop's heavier work (broadcasts, game-data refresh, title
// checks, enumeration) stays at its 500ms cadence. Without a host runner
// attached this is exactly sleepOrCancel(total): a plain scraped box pays
// nothing for the fast cadence.
//
// Sub-ticks deliberately do ONLY ReadGameState + tickHost: a state transition
// observed mid-window is acted on by the runner immediately (its Observation is
// built from the fresh read) and the loop's own transition bookkeeping
// (OnStateChange, thorough refresh, phase moves) runs on the next full
// iteration, ≤500ms later — the same latency it had before. A failed sub-read
// skips the tick (the runner never acts on a stale frame) and leaves recovery
// to the full iteration's failure gates. Loop-goroutine only.
func (r *runner) hostSubTicks(total time.Duration) {
	if r.host == nil {
		r.sleepOrCancel(total)
		return
	}
	deadline := time.Now().Add(total)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		step := hostTickMinInterval
		if remaining < step {
			step = remaining
		}
		r.sleepOrCancel(step)
		if r.ctx.Err() != nil {
			return
		}
		if time.Until(deadline) <= 0 {
			return // full iteration due — let runReady do the heavy read
		}
		gs, tick, err := r.reader.ReadGameState()
		if err != nil {
			continue
		}
		r.tickHost(gs, tick)
	}
}

// setReadout publishes the current tick's readout for the diagnostics endpoint.
// Loop goroutine only (writer).
func (r *runner) setReadout(ro hostrunner.ScraperReadout) {
	r.readoutMu.Lock()
	r.lastReadout, r.hasReadout = ro, true
	r.readoutMu.Unlock()
}

// readout returns the most recent per-tick readout. Safe from request goroutines.
func (r *runner) readout() (hostrunner.ScraperReadout, bool) {
	r.readoutMu.RLock()
	defer r.readoutMu.RUnlock()
	return r.lastReadout, r.hasReadout
}

// buildHostReadout projects the loop's current reader + game-data state into the
// hostrunner.ScraperReadout the adapter maps to an Observation. Loop-goroutine
// only (reads r.reader and r.gameData directly). Fresh is true because it's only
// called right after a successful ReadGameState; a reader read failure skips the
// call entirely, so the runner never acts on a stale frame.
func (r *runner) buildHostReadout(gs scraper.GameState, tick uint32) hostrunner.ScraperReadout {
	ro := hostrunner.ScraperReadout{
		Fresh:     true,
		Tick:      tick,
		GameState: string(gs),
	}
	if r.reader != nil {
		si := r.reader.LastStateInputs()
		ro.MenuActive = stateInputInt(si, "main_menu") != 0
		ro.GameConnection = stateInputInt(si, "game_connection")
		ro.MenuFocus = uint32(stateInputInt(si, "menu_focus"))
		ro.MenuItem = stateInputInt(si, "menu_item")
		// Screen-record classifier + UI support reads (haloce screenrec.go).
		ro.UiScreen, _ = si["ui_screen"].(string)
		ro.UiBackScreenRec = uint32(stateInputInt(si, "ui_back_screen_rec"))
		ro.UiOskActive, _ = si["ui_osk_active"].(bool)
		ro.UiMsClock = uint32(stateInputInt(si, "ui_ms_clock"))
		ro.UiFadeState = uint32(stateInputInt(si, "ui_fade_state"))
		ro.SlotClaimed, _ = si["ui_slot_claimed"].(bool)
		ro.SlotProfileHandle = uint32(stateInputInt(si, "ui_slot_profile"))
		ro.GameOver, _ = si["game_over"].(bool)
		// Raw diagnostic reads for the admin panel (string/bool, not integers).
		ro.Dela, _ = si["menu_dela"].(string)
		ro.PregameSentinel, _ = si["pregame_sentinel"].(bool)
		ro.MainMenuRaw = stateInputInt(si, "main_menu")
		ro.UIWidgetBlocks = stateInputInt(si, "ui_widget_blocks")
		ro.UIHighlighted = stateInputInt(si, "ui_highlighted")
		ro.UIMaxTick = uint32(stateInputInt(si, "ui_max_tick"))
		ro.NavCandidates, _ = si["nav_candidates"].(map[string]uint32)
	}
	ro.Map = r.gameData.Map
	ro.Gametype = r.gameData.Gametype
	ro.MachineCount = len(r.gameData.Machines)
	ro.PlayerCount = len(r.gameData.Players)
	ro.TeamCount = distinctTeams(r.gameData)
	r.fillLobbyCursor(gs, &ro)
	return ro
}

// fillLobbyCursor projects the reader's LIVE create-game carousel cursors (CE
// widget +0x4C/+0x54) into the readout. Gated to the front-end menu (gs ==
// GameStateMenu) — the SELECT MAP / SELECT GAMETYPE screens are front-end menus in
// BOTH the system-link and split-screen create paths, so this covers the live-nav
// case while keeping the ~2 MiB UI-heap scan off the in-game 30 Hz hot path.
// No-op when the bound reader can't read a cursor (title without a
// LobbyCursorReader). Off the create-game screens the widgets read count 0 →
// *Valid false, so the runner's card steps hold rather than navigating blind.
// Loop-goroutine only (reads r.reader directly).
func (r *runner) fillLobbyCursor(gs scraper.GameState, ro *hostrunner.ScraperReadout) {
	if gs != scraper.GameStateMenu || r.reader == nil {
		return
	}
	cur, ok := r.reader.(scraper.LobbyCursorReader)
	if !ok {
		return
	}
	c := cur.ReadLobbyCursor()
	ro.MapCursor, ro.MapCursorCount, ro.MapCursorValid = c.MapIndex, c.MapCount, c.MapValid
	ro.GametypeCursor, ro.GametypeCursorCount, ro.GametypeCursorValid = c.GametypeIndex, c.GametypeCount, c.GametypeValid
	// Enumerated (built-in) gametype count — the runner subtracts it from the live
	// widget count to find the custom-variant prefix (see gametypeCustomPrefix).
	r.cacheMu.Lock()
	ro.GametypeListLen = len(r.cache.AvailableGametypes)
	r.cacheMu.Unlock()
}

// stateInputInt coerces a StateInputs value (stored as its native memory width —
// uint8/uint16/uint32/…) to an int. Returns 0 for a missing or non-integer key,
// which the adapter treats as the safe menu/no-op default.
func stateInputInt(si scraper.StateInputs, key string) int {
	v, ok := si[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case uint8:
		return int(n)
	case uint16:
		return int(n)
	case uint32:
		return int(n)
	case int:
		return n
	case int8:
		return int(n)
	case int16:
		return int(n)
	case int32:
		return int(n)
	case uint64:
		return int(n)
	case int64:
		return int(n)
	default:
		return 0
	}
}

// distinctTeams estimates the number of teams present for the runner's native
// start gate (2+ teams). Prefers the distinct Team values across the roster;
// falls back to the team-score slot count when the roster is empty (a lobby
// before players are readable). The runner only TIMES the start press on this —
// Halo's own countdown still gates the actual start — so an approximate count is
// safe.
func distinctTeams(gd scraper.GameData) int {
	seen := map[uint32]struct{}{}
	for _, p := range gd.Players {
		seen[p.Team] = struct{}{}
	}
	if len(seen) > 0 {
		return len(seen)
	}
	return len(gd.TeamScores)
}
