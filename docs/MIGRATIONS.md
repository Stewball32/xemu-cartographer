# Database migrations

**Migrations are the source of truth for the schema.** Collections are created
and changed by files in [`migrations/`](../migrations), applied on boot and
tracked in PocketBase's `_migrations` table. There is no on-serve
"schema-as-code" step — that was replaced by the baseline migration.

This follows the shared standard in `stew-site-template`; cartographer adopted it
on the `beta` branch (see [The baseline](#the-baseline) for what that conversion
did).

## Why

Schema-as-code (recreating collections in an `OnServe` hook) works for *creating*
collections but has no answer for *changing* one: no ordering, no record of what
ran, no way to know if beta and prod are at the same schema, and edits made in the
admin UI silently diverge. Migrations give an ordered, reviewable, idempotent
history that every tier applies identically.

Cartographer felt this directly: the retired `schema/identity.go` had to
hand-order collection registration into numbered "phases" because PB resolves
relation targets and validates rules at save time. A snapshot import resolves all
of that itself — the phase dance is gone.

## How it's wired

- `cmd/server/main.go` registers `migratecmd` with `Dir: "migrations"` and
  blank-imports `migrations/` so the files self-register.
- **Automigrate is ON in dev builds only** — `internal/pocketbase/migrateconf`
  (`//go:build dev` → `true`, `!dev` → `false`). `task dev` / `run-dev.sh` build
  with `-tags dev`; the beta + prod snapshots do not.
  - **dev:** change the schema in the admin UI → a migration file is written to
    `migrations/` automatically. That file is what you commit.
  - **beta/prod:** never author migrations. They only *apply* what was committed.
- Pending migrations run **before** `OnServe`, so routes, hooks, the scraper
  manager and the Discord bot can all assume the schema exists.

## The model: granular in dev, squashed on release

| | dev / beta branches | `main` → prod |
| --- | --- | --- |
| Migrations | **many small files**, one per change | **one file per approved release** |
| History | messy and iterative — expected | a clean version log |
| Source | Automigrate + `migrate create` | `migrate:squash` |

**Branch discipline:** granular churn lives on dev/beta branches only. `main`
carries nothing but squashed release migrations. **The squash happens on the way
to main** — never merge granular files into it.

## Workflow

```
dev (Automigrate, many small files)
  └─► beta applies them on boot           ← iterate freely here
        └─► APPROVED → task migrate:squash   (granular ──► one release migration)
              └─► task migrate:verify        (proves squashed ≡ granular)
                    └─► merge to main ──► prod applies ONE clean migration
```

1. **Author in dev.** `./run-dev.sh`, then edit collections at
   `http://127.0.0.1:19090/_/`. A file appears in `migrations/`.
   - Prefer a blank migration for data/logic changes:
     `task migrate:create -- backfill_gamertag_sanitized`
2. **Review it.** Plain Go — confirm it does what you expect and that `down`
   actually reverses `up`.
3. **Commit it** with the code that depends on it, in the same commit.
4. **Prove it on beta.** `~/xcarto-beta/pull-beta.sh`, then start the tier —
   migrations apply on boot, so a healthy `/api/health` proves they succeeded.
5. **Deploy prod.** It applies the same pending migrations on boot.

### Squashing for a release

```sh
task migrate:squash -- --version v0.3.0     # granular ──► one release file
task migrate:verify -- --from /tmp/pb_data-copy   # PROVE it before merging
git add -A && git commit -m "chore(release): squash migrations for v0.3.0"
```

`migrate:verify` builds two trees from the *same* starting database — (A)
release-line + archived granular files, (B) release-line + the new release
migration — and compares the end schemas. Identical → safe to merge. Different →
prints the diff and exits 1.

> **The one case that legitimately fails:** if the release **deletes** a
> collection. A generated snapshot imports with `deleteMissing=false`, so it never
> removes — re-run the squash with `--delete-missing` (destructive, opt-in), then
> verify again.

## Commands

| Task | What it does |
| --- | --- |
| `task migrate:up` | Apply pending migrations (beta/prod also do this on boot) |
| `task migrate:create -- <name>` | New blank migration |
| `task migrate:collections` | Snapshot current collections into a migration (baseline / re-sync) |
| `task migrate:down` | Revert the last migration — **dev only, destructive** |
| `task migrate:squash -- --version vX.Y.Z` | Collapse granular migrations into one release migration |
| `task migrate:verify -- --from <pb_data>` | Prove the squash is faithful — **run before merging** |

Files are classified by filename: **release-line** =
`*_collections_snapshot.go` (the baseline) and `*_release_*.go`; **granular** =
everything else. `migrate:squash` archives granular files to
`.migrate-archive/<version>/` (gitignored) so `migrate:verify` can rebuild the
"before" side.

## Rules

- **Never edit an applied migration.** Add a new one.
- **Never hand-write into `_migrations`.**
- **Never merge granular migrations to `main`.** Squash first.
- **Never merge a squash that hasn't passed `migrate:verify`.**
- **One logical change per migration**, named for what it does.
- **Commit the migration with the code that needs it.**

## The baseline

`migrations/1784707177_collections_snapshot.go` is the **baseline**: a full
snapshot of all 31 collections, generated with `migrate collections` from exactly
the schema the retired `internal/pocketbase/schema` package used to create.

**How it was verified (2026-07-22):**

- A **fresh** `pb_data` booted with no schema-as-code produces a collection set
  *and* per-collection fields/rules/indexes **byte-identical** to what
  schema-as-code produced. (Diffed `_collections` between the two databases.)
- An **existing** `pb_data` (a copy of beta's) **adopts** the baseline: it is
  recorded in `_migrations` and the collections are reconciled in place — 36
  collections, 2 users and 2 superusers all preserved, nothing recreated.

It covers everything: `users`/`gamertags`/`roles`/`user_roles`, the M13 game chain
(`series`/`games`/`game_players`/`game_events`/`ratings`), profiles
(`ce_profiles`/`h2_profiles`/`gametypes`/`game_titles`), containers/`isos`, the
LAN-sync set (`apps`/`checkins`/`lan_events`/`sync_presets`), Discord
(`discord_routes`/`discord_guild_settings`), and the moderation/audit tables.

### What retiring schema-as-code dropped

The old `schema` package also carried a one-off **legacy fold** —
`discord_channel_bindings` + `discord_guilds` → `discord_routes` — which ran on
serve. That is gone. It is safe: beta already applied it (its DB has
`discord_routes` and neither legacy collection), and a fresh database gets
`discord_routes` straight from the baseline. **If a tier is ever restored from a
pre-`discord_routes` backup, that fold must be re-authored as a migration.**

The only exported Go value the package held is now
`internal/pocketbase/schemaconst.GamertagMaxLen`.
