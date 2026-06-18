// Package games owns the M13 persistence path: turning a finished scraper
// game (the Live→Ready `cache.PreviousGame` data) into PocketBase rows
// (series + games + game_players), plus the variant-name → category heuristic.
//
// Direct core.App calls, no Service interface — same reasoning as
// internal/teamperms / internal/roles: the scraper writes through this on game
// end, and the M14/M15 UIs read the rows back, but no cross-system boundary
// needs a typed interface.
package games

import "strings"

// Category constants mirror the `series.category` SelectField.
const (
	CategoryCasual      = "casual"
	CategoryCompetitive = "competitive"
	CategoryTournament  = "tournament"
	CategoryCustom      = "custom"
)

// competitiveMarkers are case-insensitive substrings in a gametype variant
// name that suggest a competitive series (M13c). The `tournament` category is
// set explicitly by M14 series setup, not inferred here.
var competitiveMarkers = []string{"tournament", "mlg", "comp", "hcs", "pro"}

// SuggestCategory maps a gametype variant name to a suggested series category.
// Competitive markers ("MLG Tournament v7", "HCS Slayer", "Comp CTF") →
// "competitive"; everything else defaults to "casual". An explicit override
// from M14 series setup beats this; admins can re-categorize anytime through
// the PB admin UI.
func SuggestCategory(variantName string) string {
	v := strings.ToLower(strings.TrimSpace(variantName))
	if v == "" {
		return CategoryCasual
	}
	for _, m := range competitiveMarkers {
		if strings.Contains(v, m) {
			return CategoryCompetitive
		}
	}
	return CategoryCasual
}
