# M32 — xc-scraper daemon + wire-only cut-over (step 8)

> **Status:** In progress
> **Started:** 2026-09-07
> **Completed:** —
> **Depends on:** M31 (pre-freeze hardening; the frozen wire contract), M05 (phase model / envelope classes), M13 (game-end persistence chain)

## Goal

Run the scraper as its own process. Step 7 moved the runner and the wire contract into
the sibling module `github.com/xemu-cartographer/xc-scraper`; step 8 wraps them in a
daemon (`cmd/xc-scraper`) that attaches to the xemu instances, serves the **frozen v1
wire feed** over its own WebSocket, and delivers finished games by webhook — and
teaches the league to consume that feed instead of embedding the runner. The point is
decoupling: the feed can be developed, restarted and deployed without PocketBase in
the loop, other consumers (overlays, OBS, a second league) can read the same feed, and
the league's own release cadence stops being tied to memory-reader changes. The wire
dialect does not change — browsers and the vendored TS types are untouched (D-2). The
design is `../xemu-cartographer-split/scraper-repo-design/DESIGN-STEP8.md`; the
decision record is [ADR-0005](../decisions/0005-wire-only-scraper-cutover.md).

## Scope

In:

- **The daemon** (xc-scraper repo): `cmd/xc-scraper` + `hub/` (trimmed WS server) +
  `daemon/` (attach loop, layered config, read + control API) + `gameout/` (spool-first
  finished-game webhook), TCP QMP, the in-daemon host runner behind `--hostrunner`.
- **The league consumer** (this repo): `internal/xcclient` (stream client + mirror),
  `leaguescraper.Boot` / `Adapter` / demand observer / byte-identical rebroadcast /
  config pusher / `pb:` event writer, `POST /api/xc/finished_game` ingest, proxied admin
  routes, the reaper's mirror source.
- **R1 dual mode**: `XC_SCRAPER_URL` set ⇒ wire mode; unset ⇒ in-process exactly as
  before. Both paths stay in the tree until R2.
- **Dev loop, build, deployment, CI, docs**: `task dev` runs the daemon (Air, port
  8992) next to the backend and frontend; `task build` yields `bin/server` +
  `bin/xc-scraper`; systemd unit template + install notes in `../xc-scraper/deploy/`;
  `-race` on the two concurrent packages in CI; this milestone, ADR-0005, RUNBOOK,
  DEPLOYMENTS, websocket-api, CHANGELOG entries.

Out (R2 and later, or owner gates):

- Deleting the in-process path (R2 — a separate milestone after pre soaks on wire
  mode).
- Pushing the xc-scraper repo to GitHub, enabling its Actions, claiming ports
  `8990-8992` in `/srv/registry/PORTS.md`, installing the units, extending
  `srv-pre.sh` with the sibling checkout + daemon install (all owner-only, §17).
- Any wire dialect change (frozen; the `xc:` room family is the only extension).

## Actions

- [x] xc-scraper: `cmd/xc-scraper` flags/env twins, boot line, `--version`.
- [x] xc-scraper: `hub/` trimmed WS server, token mode, `xc:` rooms.
- [x] xc-scraper: `daemon/` attach loop, layered config + `control.json` (D-10), read API, control API.
- [x] xc-scraper: `gameout/` spool-first webhook, `--game-dir`, stdout sink.
- [x] xc-scraper: TCP QMP (`--tcp`, `--watch-ports`), in-daemon host runner.
- [x] league: `internal/xcclient` stream client + mirror, `leaguescraper.Boot` wire mode.
- [x] league: `POST /api/xc/finished_game` ingest, proxied routes, config pusher, event writer.
- [x] league: reaper mirror source, R1 boot line, RUNBOOK switch/rollback.
- [x] `task dev` / `task build` / CI / deploy unit / docs (D1, 2026-09-07).
- [ ] Owner: push xc-scraper, enable Actions, register ports, install `xc-scraper-pre`, extend `srv-pre.sh`.
- [ ] Soak pre on wire mode (`XC_SCRAPER_URL` set) through at least one LAN night.
- [ ] R2: remove the in-process path (new milestone).

## Verification

- `task build` produces `bin/server` and `bin/xc-scraper`; `bin/xc-scraper --version`
  prints the `git describe` string.
- `task dev` prints three Air/Vite banners; `curl 127.0.0.1:8992/api/health` answers
  `{"ok":true,…}`; the league boot line reads `leaguescraper: mode=wire url=http://127.0.0.1:8992 …`.
- The vendored wire artifacts are byte-identical (`task sync-wire:check`); the golden
  fixtures under `../xc-scraper/wire/testdata` are unchanged.
- `go test -race ./internal/websocket/... ./internal/xcclient/...` and
  `cd ../xc-scraper && go test -race ./hub/... ./daemon/... ./runner/... ./gameout/...`
  pass.
- A finished game reaches the `games` collection through
  `POST /api/xc/finished_game` (unit-tested with a fixture; live proof needs xemu).
- On pre: the same admin pages (pod list, debug tabs, host control) work with
  `XC_SCRAPER_URL` set; rolling back per the RUNBOOK leaves no duplicated games.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-09-07: created (D1 slice of the step-8 build). Daemon, league consumer and
  the dev/build/deploy/CI surfaces landed on `feat/xc-restructure` + xc-scraper
  `main`; owner gates (push, ports, units) still open.
- 2026-09-14: step-8 polish (flagship) — shutdown sends `1001 league shutdown`
  instead of dropping the socket, `GET /api/admin/scraper/upstream` gains
  `attempts` / `last_error` / `auth_rejected` (a refused `XC_SCRAPER_TOKEN` no
  longer looks like a healthy silent stream), and `/admin/pod/` + `/admin/pod/<name>/`
  show the §12 upstream banner (`UpstreamBanner.svelte`, 5 s poll, wire mode only).
