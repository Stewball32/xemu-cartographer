package audit

// ActionBan records an admin blocking a user from authentication (M08f).
// Distinct from ActionBlock — ActionBlock targets a single row (gamertag, team
// name), ActionBan targets the user as a whole and blocks every auth method.
// The complementary ActionUnban lifts the block; for time-bounded blocks use
// ActionTimeout.
const ActionBan Action = "ban"

// BanPayload accompanies an ActionBan audit row. Reason captures the admin's
// justification. ByMigration is reserved for future synthetic backfills (no
// pre-existing ban data to migrate in M08, so all real ban rows ship with
// ByMigration=false).
type BanPayload struct {
	Reason      string `json:"reason,omitempty"`
	ByMigration bool   `json:"by_migration,omitempty"`
}
