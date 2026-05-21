package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectRoster emits game_update events when a roster slot changes
// between ticks (kind=player_joined / player_left / player_team_changed)
// and per-team events when a team's score changes (kind=team_score).
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
			ref := playerRefFromGame(cur)
			out = append(out, ctx.emitGameUpdate(scraper.GameUpdateEvent{
				Kind:   scraper.GameUpdateKindPlayerJoined,
				Player: &ref,
			}))
			continue
		}
		if prev.Team != cur.Team {
			ref := playerRefFromGame(cur)
			prevTeam := prev.Team
			out = append(out, ctx.emitGameUpdate(scraper.GameUpdateEvent{
				Kind:         scraper.GameUpdateKindPlayerTeamChanged,
				Player:       &ref,
				PreviousTeam: &prevTeam,
			}))
		}
		_ = idx
	}

	// Leaves: anything in PrevRoster missing from currByIdx. ArmorColor
	// is left zero — RosterEntry doesn't carry it; v1 didn't either.
	for idx, prev := range ctx.State.PrevRoster {
		if _, stillThere := currByIdx[idx]; stillThere {
			continue
		}
		ref := scraper.PlayerRef{
			Index: idx,
			Name:  prev.Name,
			Team:  prev.Team,
		}
		out = append(out, ctx.emitGameUpdate(scraper.GameUpdateEvent{
			Kind:   scraper.GameUpdateKindPlayerLeft,
			Player: &ref,
		}))
	}

	// Team scores: one team_score event per team whose score changed.
	for _, ts := range ctx.Snap.TeamScores {
		if prevScore, ok := ctx.State.PrevTeamScores[ts.Team]; ok {
			if prevScore == ts.Score {
				continue
			}
		}
		team := ts.Team
		score := ts.Score
		out = append(out, ctx.emitGameUpdate(scraper.GameUpdateEvent{
			Kind:  scraper.GameUpdateKindTeamScore,
			Team:  &team,
			Score: &score,
		}))
	}

	return out
}

func updateRosterPrev(state *scraper.TickState, result scraper.TickResult) {
	// Bookkeeping happens inline at the end of detectRoster via
	// finalizeRosterPrev, since the diff source is game data (not result).
	_ = state
	_ = result
}

// finalizeRosterPrev records this game data's roster + team scores for
// the next tick's diff. Inlined rather than via Updater because Updater
// signature doesn't receive game data.
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
