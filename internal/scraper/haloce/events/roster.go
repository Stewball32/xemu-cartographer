package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectRoster emits per-player events when a roster slot changes between
// ticks (player_joined / player_left / player_team_changed) and per-team
// events when a team's score changes (team_score). All four diff against
// the game data — they're refreshed every loop iteration via ReadReadyState, so
// changes show up within ~500ms in pregame and within one game tick in_game.
//
// player_team_changed is independent of player_joined / player_left: it
// fires only when the same index existed in both ticks with a different
// Team value. A roster slot replacement (player A leaves, player B takes
// the same index) emits player_left + player_joined back-to-back rather
// than player_team_changed.
func detectRoster(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope
	currByIdx := gamePlayerByIndex(ctx.Snap)

	// Joins + team changes.
	for idx, cur := range currByIdx {
		prev, existed := ctx.State.PrevRoster[idx]
		if !existed {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventPlayerJoined,
				"player":     idx,
				"name":       cur.Name,
				"team":       cur.Team,
			}))
			continue
		}
		if prev.Team != cur.Team {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventPlayerTeamChanged,
				"player":     idx,
				"prev_team":  prev.Team,
				"team":       cur.Team,
			}))
		}
	}

	// Leaves: anything in PrevRoster missing from currByIdx.
	for idx, prev := range ctx.State.PrevRoster {
		if _, stillThere := currByIdx[idx]; stillThere {
			continue
		}
		out = append(out, ctx.emit(map[string]any{
			"event_type": scraper.EventPlayerLeft,
			"player":     idx,
			"name":       prev.Name,
			"team":       prev.Team,
		}))
	}

	// Team scores: emit one team_score event per team whose score changed.
	for _, ts := range ctx.Snap.TeamScores {
		if prev, ok := ctx.State.PrevTeamScores[ts.Team]; ok {
			if prev == ts.Score {
				continue
			}
		}
		out = append(out, ctx.emit(map[string]any{
			"event_type": scraper.EventTeamScore,
			"team":       ts.Team,
			"score":      ts.Score,
		}))
	}

	return out
}

func updateRosterPrev(state *scraper.TickState, result scraper.TickResult) {
	// Roster updates are driven by game data, not result. The loop calls
	// Detect with snap=ctx.Snap which the manager assigns to runner.gameData
	// before each Detect call; result is the tick payload. To update PrevRoster
	// from the game data we'd need it here too — but Updater takes only
	// (state, result). Workaround: roster.go performs its own bookkeeping
	// inline at the end of detectRoster (state mutation is safe — single
	// goroutine, called once per tick). updateRosterPrev is a no-op kept for
	// symmetry with other event modules.
	_ = state
	_ = result
}

// finalizeRosterPrev is called by detectRoster at the end of every dispatch
// to record this game data's roster + team scores for the next tick's diff.
// Inlined here rather than via Updater because it depends on the game data,
// which Updater functions don't receive.
func finalizeRosterPrev(state *scraper.TickState, snap scraper.GameData) {
	state.PrevRoster = make(map[int]scraper.RosterEntry, len(snap.Players))
	for _, p := range snap.Players {
		state.PrevRoster[p.Index] = scraper.RosterEntry{Name: p.Name, Team: p.Team}
	}
	state.PrevTeamScores = make(map[uint32]int32, len(snap.TeamScores))
	for _, ts := range snap.TeamScores {
		state.PrevTeamScores[ts.Team] = ts.Score
	}
}

func init() {
	RegisterDetector(func(ctx *Context) []scraper.Envelope {
		out := detectRoster(ctx)
		finalizeRosterPrev(ctx.State, ctx.Snap)
		return out
	})
	RegisterUpdater(updateRosterPrev) // no-op; bookkeeping happens inline
}
