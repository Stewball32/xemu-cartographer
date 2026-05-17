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

- golangci-lint sweep across `internal/`: explicitly drop ignored `Close()` errors, return wrapped errors on `fmt.Fprintln` writes, drop dead helpers (`playerRefPtr`, `vehicleRefPtr`, `itemRefPtr`, `intPtr` and friends, `strPtr`, `vec3FromXYZ`, `readyBroadcastInterval`, test-only `fakeReader`), and document `//nolint:unused` registration scaffolding.
- `internal/scraper/haloce/events/vehicle.go`: rewrite `!(prevAlive && tp.Alive)` as `!prevAlive || !tp.Alive` for staticcheck.
- Regenerated PocketBase TypeScript types to include the `capture_policies` and `game_events` collections introduced in earlier v2-31 / v2-18 commits.
- Xbox debug tab is now a pure view of the `xbox` envelope: dropped the leftover runtime/transport sections (Connection / Lifecycle / Cross-instance summary / Envelope freshness) that mixed transport state with envelope contents.
- `DebugContext` gains optional `viewMode` + `setViewMode`; the probe page no longer needs to opt in.

### Deprecated

### Removed

- `SystemTab.svelte`, `runtime-vm.ts`, and the old per-tab `xbox-vm.ts` under `sveltekit/src/lib/components/debug/system/` (folder removed entirely — superseded by `debug/xbox/`).
- Unused `showAll` getter from the probe page's `DebugContext` wiring.

### Fixed

### Fixed

- `apiBaseURL()` now honors the page's `https:` protocol in dev (was hardcoded `http`).

### Security
