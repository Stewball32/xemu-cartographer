//go:build !dev

package migrateconf

// Automigrate is OFF in beta/prod builds.
//
// These tiers apply committed migrations on boot and must never generate new
// ones from live admin edits. Schema changes are authored in dev (where
// Automigrate is true), committed, then applied here. See docs/MIGRATIONS.md.
const Automigrate = false
