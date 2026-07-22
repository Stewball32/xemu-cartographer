// Package migrations holds the PocketBase database migrations that define
// cartographer's schema. It is the SOURCE OF TRUTH for collections — see
// docs/MIGRATIONS.md for the workflow.
//
// Files here self-register via init() + m.Register(up, down) and are applied in
// filename (timestamp) order. cmd/server/main.go blank-imports this package so
// they load; pending ones are applied on boot and recorded in the _migrations
// table, BEFORE OnServe — so routes/hooks/the scraper can assume the schema
// exists.
//
// This replaced the old on-serve "schema-as-code" step
// (internal/pocketbase/schema/*.go + its identity.go phase ordering): the
// baseline snapshot below was generated from exactly the schema that code
// produced, so a fresh pb_data builds the identical collection set from
// migrations alone.
//
// Author migrations in dev (Automigrate writes them when you change the schema
// in the admin UI), review the generated file, commit it, then let beta/prod
// apply it. Never hand-edit an already-applied migration — add a new one.
package migrations
