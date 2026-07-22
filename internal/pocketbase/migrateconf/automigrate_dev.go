//go:build dev

package migrateconf

// Automigrate is ON in dev builds only.
//
// With it enabled, any collection change you make in the PocketBase admin UI is
// written out as a migration file under migrations/ automatically — that file is
// what you commit. See docs/MIGRATIONS.md.
//
// It is deliberately OFF in beta/prod builds (automigrate_prod.go): those tiers
// must only ever APPLY the migrations that were committed and reviewed, never
// author new ones from a live admin edit.
//
// `task dev` / air build with `-tags dev`; the beta + prod snapshots do not.
const Automigrate = true
