package embeds

import (
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/discord"

	"github.com/Stewball32/xemu-cartographer/internal/rating"
)

// Leaderboard renders a per-game-type rating leaderboard (M18c/d). entries are
// already ranked (highest first).
func Leaderboard(gametype string, entries []rating.Entry) discord.Embed {
	e := discord.Embed{Title: "Leaderboard — " + gametype, Color: 0x5865f2}
	if len(entries) == 0 {
		e.Description = "No rated players yet."
		return e
	}
	var b strings.Builder
	for i, en := range entries {
		fmt.Fprintf(&b, "**%d.** %s — %.0f (%d games)\n", i+1, en.Gamertag, en.Rating, en.Games)
	}
	e.Description = b.String()
	return e
}

// Rank renders a single player's rank in a game type (M18d /rank). rank==0 means
// the player is unrated.
func Rank(gamertag, gametype string, rank int, ratingValue float64) discord.Embed {
	e := discord.Embed{Title: "Rank — " + gamertag, Color: 0x5865f2}
	if rank == 0 {
		e.Description = "Unrated in " + gametype + "."
		return e
	}
	e.Description = fmt.Sprintf("**#%d** in %s · %.0f rating", rank, gametype, ratingValue)
	return e
}
