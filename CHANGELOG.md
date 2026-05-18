# Changelog

All notable changes to this project are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `/api/version` endpoint returning the git-tag-derived version, short commit, and build date.
- Container images now tagged with both `:VERSION` and `:latest`.
- ADR-0001 documents the single-source-of-truth version pipeline (git tag → ldflags → `internal/version` → `/api/version` → frontend `PUBLIC_APP_VERSION`).
- Standard `task test` / `task fmt` / `task lint` targets matching the template's Taskfile convention.
- Overview debug tab: Handshake section surfacing the v2 hello envelope (`protocol_version`, `server_time`, advertised classes, known instances).
- Placeholder debug tabs for `container`, `debug`, `objects`, `scenario`, `summary` so the tab strip reflects the planned M6c structure (M06).
- Xbox debug tab: envelope-stats header (`seq`, `tick`, `received`, `v`, `instance`), Pretty/JSON view toggle (Skeleton `Switch` with `LayoutGrid`/`Braces` icons), and a JSON view with Skeleton `TreeView` navigation + scoped `CodeBlock` (selecting a node returns the envelope wrapped down to that path) (M06).
- Page-level `viewMode` preference on the debug page (Pretty / JSON), persisted to `localStorage['debug.view']` so the choice carries across tabs that opt into the same toggle (M06).
- WS v2 store exposes `xboxEnvelope[instance]` (full `EnvelopeV2<XboxPayload>` including `seq`/`tick`/`ts`/`v`) alongside the existing payload-only `xbox[instance]` slot.

### Changed

- Game debug tab rewritten on the xbox Pretty/JSON pattern: envelope-stats header (`seq`/`tick`/`received`/`v`/`instance`) with the shared page-level view toggle, Pretty view as a Skeleton `Accordion` of `top-level` / `config` / `team_scores` / `players` / `machines` / `network` sections (tiles for nested scalars, `DataTable` for array sections, every field always rendered with `—` for missing), JSON view with Skeleton `TreeView` + scoped `CodeBlock`. WS v2 store exposes `gameEnvelope[instance]` alongside the existing payload-only `game[instance]`. Old per-section components (FogSection, GameInfoSection, MachinesSection, ObjectTypesSection, PlayersSection, PowerItemsSection, SpawnsSection, TagCacheSection) removed. The `v2ToV1GameData` adapter still consumed by overview + postgame vms moved to `debug/shared/v2-to-v1-game.ts` (M06).
- Debug envelope tab migrated from placeholder to the xbox Pretty/JSON pattern: envelope-stats header (`seq`/`tick`/`received`/`v`/`instance`), Pretty view with Accordion sections (`players`, `state_inputs`, `score_probe`) rendering every expected field with `—` for missing and distinct `present`/`null`/`missing` tags for the nullable `extended`/`bones`/`update_queue`/`raw` members of `DebugPlayer`, and a JSON view (Skeleton `TreeView` + scoped `CodeBlock`). WS v2 store exposes `debugEnvelope[instance]` alongside the existing payload-only `debug[instance]` slot (M06).
- golangci-lint sweep across `internal/`: explicitly drop ignored `Close()` errors, return wrapped errors on `fmt.Fprintln` writes, drop dead helpers (`playerRefPtr`, `vehicleRefPtr`, `itemRefPtr`, `intPtr` and friends, `strPtr`, `vec3FromXYZ`, `readyBroadcastInterval`, test-only `fakeReader`), and document `//nolint:unused` registration scaffolding.
- `internal/scraper/haloce/events/vehicle.go`: rewrite `!(prevAlive && tp.Alive)` as `!prevAlive || !tp.Alive` for staticcheck.
- Regenerated PocketBase TypeScript types to include the `capture_policies` and `game_events` collections introduced in earlier v2-31 / v2-18 commits.
- Xbox debug tab is now a pure view of the `xbox` envelope: dropped the leftover runtime/transport sections (Connection / Lifecycle / Cross-instance summary / Envelope freshness) that mixed transport state with envelope contents.
- `DebugContext` gains optional `viewMode` + `setViewMode`; the probe page no longer needs to opt in.
- Scenario debug tab migrated from a placeholder to the full xbox Pretty/JSON pattern: envelope-stats header + Pretty Accordion (`top-level`, `fog`, `memory_regions`, `object_types`, `player_spawns`, `power_item_spawns`, `tag_defs`) with always-rendered tiles + `DataTable` lists, plus a `TreeView`-driven JSON view scoped to the selected node. WS v2 store now exposes `scenarioEnvelope[instance]` alongside the existing payload-only `scenario[instance]` (M06).
- Tick debug tab rewritten on the xbox Pretty/JSON pattern: envelope-stats header (`seq`/`tick`/`received`/`v`/`instance`) with the Pretty/JSON Switch, Pretty view as an Accordion of five sections that mirror the JSON shape (`players` / `power_items` / `ctf_flags` / `game_globals` / `locals`) with every field always rendered, and a JSON view that pairs a Skeleton `TreeView` with a scoped `CodeBlock`. The v2→v1 tick projection that fed the legacy v1 sections moved into `overview-vm.ts` (the only remaining consumer) so `tick-vm.ts` is now a pure formatter (M06). WS v2 store exposes `tickEnvelope[instance]` alongside the existing payload-only `tick[instance]` slot.
- Objects debug tab migrated from the placeholder shell to the xbox Pretty/JSON pattern: envelope-stats header (`seq`/`tick`/`received`/`v`/`instance`), Accordion Pretty view with `summary`/`objects`/`projectiles` sections (DataTables for the two arrays, count tiles in the summary), and a Skeleton `TreeView` + scoped `CodeBlock` JSON view. WS v2 store exposes `objectsEnvelope[instance]` alongside the payload-only `objects[instance]` slot (M06).

### Deprecated

### Removed

- `SystemTab.svelte`, `runtime-vm.ts`, and the old per-tab `xbox-vm.ts` under `sveltekit/src/lib/components/debug/system/` (folder removed entirely — superseded by `debug/xbox/`).
- Unused `showAll` getter from the probe page's `DebugContext` wiring.

### Fixed

### Fixed

- `apiBaseURL()` now honors the page's `https:` protocol in dev (was hardcoded `http`).

### Security
