package rooms

import "github.com/Stewball32/xemu-cartographer/internal/guards"

// Clients in the "overlay" room receive scraper snapshot/tick/event broadcasts
// from internal/scraper/manager. RequireAuth ensures only logged-in PocketBase
// users (any role) can subscribe; tighten to RequireAdmin if overlay payloads
// ever carry sensitive data (e.g. anti-cheat telemetry).
func init() {
	register(&RoomType{
		Name:   "overlay",
		Guards: []GuardFunc{guards.RequireAuth},
	})
}
