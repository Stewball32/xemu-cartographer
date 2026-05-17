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

### Changed

- golangci-lint sweep across `internal/`: explicitly drop ignored `Close()` errors, return wrapped errors on `fmt.Fprintln` writes, drop dead helpers (`playerRefPtr`, `vehicleRefPtr`, `itemRefPtr`, `intPtr` and friends, `strPtr`, `vec3FromXYZ`, `readyBroadcastInterval`, test-only `fakeReader`), and document `//nolint:unused` registration scaffolding.
- `internal/scraper/haloce/events/vehicle.go`: rewrite `!(prevAlive && tp.Alive)` as `!prevAlive || !tp.Alive` for staticcheck.
- Regenerated PocketBase TypeScript types to include the `capture_policies` and `game_events` collections introduced in earlier v2-31 / v2-18 commits.

### Deprecated

### Removed

### Fixed

### Fixed

- `apiBaseURL()` now honors the page's `https:` protocol in dev (was hardcoded `http`).

### Security
