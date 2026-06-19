package embeds

import (
	"fmt"
	"sort"

	"github.com/disgoorg/disgo/discord"

	"github.com/Stewball32/xemu-cartographer/internal/stats"
)

func boolPtr(b bool) *bool { return &b }

// UserStats renders a player's rolled-up stats (M17b /stats user). byType is the
// per-game-type split; its keys are sorted for deterministic output.
func UserStats(gamertag string, total stats.Totals, byType map[string]stats.Totals) discord.Embed {
	e := discord.Embed{
		Title: "Stats — " + gamertag,
		Color: 0x5865f2,
		Fields: []discord.EmbedField{
			{Name: "Games", Value: fmt.Sprintf("%d", total.Games), Inline: boolPtr(true)},
			{Name: "W / L", Value: fmt.Sprintf("%d–%d · %.0f%%", total.Wins, total.Losses, total.WinRate()*100), Inline: boolPtr(true)},
			{Name: "K / D / A", Value: fmt.Sprintf("%d / %d / %d · %.2f KD", total.Kills, total.Deaths, total.Assists, total.KD()), Inline: boolPtr(true)},
		},
	}

	gts := make([]string, 0, len(byType))
	for gt := range byType {
		gts = append(gts, gt)
	}
	sort.Strings(gts)
	for _, gt := range gts {
		t := byType[gt]
		e.Fields = append(e.Fields, discord.EmbedField{
			Name:  gt,
			Value: fmt.Sprintf("%d games · %.2f KD · %.0f%% WR", t.Games, t.KD(), t.WinRate()*100),
		})
	}

	if total.Games == 0 {
		e.Description = "No recorded games yet."
	}
	return e
}

// RecentGames renders a player's most recent games (M17b /recent). lines are
// newest-first (as PlayerGamesForGamertag returns them); at most `limit` are
// shown. Map/date aren't on the stats projection yet — gametype + result + K/D
// is what's available (an M15 follow-up can enrich PlayerGame).
func RecentGames(gamertag string, lines []stats.PlayerGame, limit int) discord.Embed {
	if limit <= 0 {
		limit = 10
	}
	e := discord.Embed{Title: "Recent games — " + gamertag, Color: 0x5865f2}
	if len(lines) == 0 {
		e.Description = "No recorded games yet."
		return e
	}
	n := len(lines)
	if n > limit {
		n = limit
	}
	for _, l := range lines[:n] {
		result := "L"
		if l.Won {
			result = "W"
		}
		e.Fields = append(e.Fields, discord.EmbedField{
			Name:  fmt.Sprintf("%s — %s", result, l.Gametype),
			Value: fmt.Sprintf("%d / %d K/D", l.Kills, l.Deaths),
		})
	}
	return e
}
