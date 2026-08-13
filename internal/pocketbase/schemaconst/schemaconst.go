// Package schemaconst holds the handful of schema VALUES that Go code needs to
// know about now that the schema itself lives in migrations/ (see
// docs/MIGRATIONS.md). It is plain constants only — no PocketBase types, no
// collection creation. It replaced the exported surface of the retired
// internal/pocketbase/schema package.
//
// Rule of thumb: a value belongs here only if Go code (or a mirrored frontend
// constant) has to agree with the schema. Collection and field NAMES are
// referenced as string literals at their call sites (and, where a package owns
// one collection, as a private const in that package — e.g. discordcfg's
// bindingsCollection); they are not centralised here.
//
// The collections themselves — fields, indexes, and PB rules (including the M08
// hasAdminRole / organizerOrAdmin subqueries that used to live in schema/rules.go)
// — are defined by migrations/*_collections_snapshot.go and any later migration.
package schemaconst

// GamertagMaxLen caps users.gamertag — the in-game name, SEPARATE from the
// account username. The gamertag is written into BOTH Halo player profiles, so
// it is capped to Halo: CE's tighter limit.
//
// Mirrored on the frontend by GAMERTAG_MAX_LEN in
// sveltekit/src/lib/utils/gamertag.ts — keep the two in sync, and change the
// users.gamertag field's Max in a migration if this ever moves.
const GamertagMaxLen = 11
