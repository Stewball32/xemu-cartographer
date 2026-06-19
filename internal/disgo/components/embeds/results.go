package embeds

import (
	"fmt"

	"github.com/disgoorg/disgo/discord"
)

// GameResultInput is the data a finished-game post needs (M17c). Built by the
// post hook from the games record (+ its series category).
type GameResultInput struct {
	Map          string
	Gametype     string
	VariantName  string
	ScoreSummary string
	Category     string
	WinnerTeam   *int
}

// GameResult renders a finished-game result embed (M17c event posting). Green.
func GameResult(in GameResultInput) discord.Embed {
	title := "Game finished"
	if in.Map != "" {
		title += " — " + in.Map
	}
	e := discord.Embed{Title: title, Color: 0x57f287}

	addField := func(name, value string, inline bool) {
		if value == "" {
			return
		}
		e.Fields = append(e.Fields, discord.EmbedField{Name: name, Value: value, Inline: boolPtr(inline)})
	}
	gt := in.Gametype
	if in.VariantName != "" {
		gt = in.VariantName
	}
	addField("Gametype", gt, true)
	addField("Score", in.ScoreSummary, true)
	if in.WinnerTeam != nil {
		addField("Winner", fmt.Sprintf("Team %d", *in.WinnerTeam), true)
	}
	addField("Category", in.Category, true)
	return e
}

// SeriesResult renders a completed-series result (M14/M17c). teamWins is per-team
// game wins; winnerTeam is the deciding team (or nil for a draw / undecided).
func SeriesResult(name, format string, teamWins []int, winnerTeam *int) discord.Embed {
	title := "Series complete"
	if name != "" {
		title += " — " + name
	}
	e := discord.Embed{Title: title, Color: 0x57f287}
	score := ""
	for i, w := range teamWins {
		if i > 0 {
			score += " – "
		}
		score += fmt.Sprintf("%d", w)
	}
	if score != "" {
		e.Fields = append(e.Fields, discord.EmbedField{Name: "Score", Value: score, Inline: boolPtr(true)})
	}
	if format != "" {
		e.Fields = append(e.Fields, discord.EmbedField{Name: "Format", Value: format, Inline: boolPtr(true)})
	}
	if winnerTeam != nil {
		e.Fields = append(e.Fields, discord.EmbedField{Name: "Winner", Value: fmt.Sprintf("Team %d", *winnerTeam), Inline: boolPtr(true)})
	}
	return e
}

// TournamentAnnounce renders a new-tournament / round-update announcement
// (M17d). The posting + the M16 tournaments-schema wiring are deferred (no
// tournaments collection yet) — this builder is ready for when they land.
func TournamentAnnounce(name, line string) discord.Embed {
	return discord.Embed{
		Title:       "🏆 " + name,
		Description: line,
		Color:       0xfee75c,
	}
}
