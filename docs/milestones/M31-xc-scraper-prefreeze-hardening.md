# M31 — xc-scraper pre-freeze hardening (extraction prep)

> **Status:** In progress
> **Started:** 2026-08-31
> **Completed:** —
> **Depends on:** M05 (phase model / envelope classes), M13 (game-end persistence chain), M20 (offset resolver plumbing)

## Goal

Fix the live bugs and contract landmines in the scraper stack **before** its wire
contract freezes and the scraper is extracted into its own repo
(`github.com/xemu-cartographer/xc-scraper`). Every item is a real improvement to
the monolith today, independent of the split — but once an external consumer
builds against the wire, each of these would be frozen in as either a
compatibility burden or a data-integrity trap (double-applied Elo, phantom
finished games, permanently dead runners). Design + evidence live in the local
split workspace: `xemu-cartographer-split/scraper-repo-design/DESIGN.md`
("Pre-freeze fix list"; line-level evidence in `protocol.md`,
`consumerCritique.md`, `managerSplit.md`, `techCritique.md` beside it — local
reference material, not in-repo).

## Scope

The 8-item pre-freeze fix list from DESIGN.md:

1. **Per-match event log** — `previous_game.events` complete + oldest-first
   (10 000 cap + `events_truncated`), separate from the 50-entry
   `request_events` ring.
2. **Class lists** — `hello.classes` + capture-sink `allClasses` share one
   registry including `event` / `game_filtered` / `event_filtered`.
3. **Anonymous-identity resync bug** — per-sender Rooms capability replaces
   `UserRooms(UserID)` in `request_state` / `request_events` / `request_probe`.
4. **Artifact lifecycle** — `end_reason` (`postgame` / `left_match` /
   `shutdown`) + synchronous bounded persist flush on shutdown + the documented
   "recorded only if the scraper observes the end" rule.
5. **`game_uid`** — minted once in `captureLiveAsPrevious`, stable across
   join-replay re-deliveries; the consumer idempotency key.
6. **offsets** — `RegisterBaseline` plugin registration + error-returning
   `Baseline`/`Resolve` (no more runner-killing panic on unknown games).
7. **QMP deadlines** — dial + per-command timeouts so a wedged xemu can't hang
   a runner forever.
8. **League-side ingest hardening** — `PersistFinishedGame` transactional +
   idempotent on a `games.game_uid` partial unique index.

Out of scope: DESIGN.md's item 9 (`--sock` retry / duplicate-basename
rejection — follow-up), the in-repo restructure / port-threading, `wire/`
extraction, the physical repo cut, and any ADR for the split (that comes when
the repo split actually happens).

## Actions

- [x] Per-match event log (manager dual-log; capture moves the full log;
      cleared on xemu death — no phantom artifact)
- [x] Shared class registry (`manager/classes.go`) feeding hello + sinks;
      pinned both ways against the rooms table (`rooms.ScraperClasses`); the
      admin capture-policies picker lists the two filtered classes
- [x] Per-sender `Rooms()` capability on `handlers.Event`; the three resync
      handlers fail closed without it
- [x] `end_reason` derivation at the `runLive` exit + WG-tracked persist
      goroutine with bounded (5s) flush in `Stop`
- [x] `newGameUID()` (unix-ms + crypto/rand, 32 hex) minted once at capture
- [x] `offsets.RegisterBaseline` + error-returning `Baseline`/`Resolve`;
      manager degrades to Idle-with-retry on unknown games
- [x] QMP dial/command deadlines (`internal/xemu/qmp.go`) + per-instance
      overrides; timeouts diagnosable via `os.ErrDeadlineExceeded`
- [x] Transactional `PersistFinishedGame` + `game_uid`/`end_reason` columns +
      partial unique index migration; dedupe observable via `Result.Deduped`
- [x] Docs/types brought in line: `docs/websocket-api.md` (classes,
      previous_game contract), `scraper-v2.ts` additive fields, CHANGELOG
- [ ] Live pass: a real match end-to-end on xemu confirming `game_uid` /
      `end_reason` / full event log on the wire and dedupe on redelivery
- [ ] DESIGN.md item 9 follow-up (`--sock` retry + duplicate socket basename
      rejection) or explicit deferral to the split work
- [ ] Decide how partial artifacts count: `docs/websocket-api.md` tells wire
      consumers to gate ratings on `end_reason`, but the in-repo chain rates
      every persisted artifact (`left_match` / `shutdown` included). Owner
      call before a real season — skip, or rate only `postgame`

## Verification

- `go vet ./...` clean; `go test ./internal/scraper/... ./internal/websocket/...
  ./internal/games/... ./internal/xemu/...` green (manager + games also under
  `-race`); repo-wide `go build ./...` green; `pnpm check` green.
- Regression tests pin each fix: cross-replay between two anonymous clients
  (fails against the pre-fix handlers), 10k truncation + flag, uid stability
  across replay builds, end-reason table, xemu-death → no artifact, sink-opens
  for the three revived classes, hello == registry == rooms table, QMP
  wedged-monitor timeout, 8-goroutine same-uid persist race → exactly one row +
  one Elo application, and a forced concurrent insert → the loser trips the
  unique index, rolls back, and reports the winner's row as `Deduped`.
- Final proof is the live pass above (needs a running xemu match).

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-08-31: created. All eight pre-freeze fixes landed together on
  `feat/xc-prefreeze-hardening` (scraper manager, websocket handlers/hub,
  offsets, xemu QMP, games persistence + migration
  `1788211877_games_ingest_dedupe.go`), plus the docs/types alignment (this
  file, `docs/websocket-api.md`, `sveltekit/src/lib/types/scraper-v2.ts`,
  CHANGELOG). Discoveries recorded for the split: `Hub.UserRooms` / the
  `websocket.Service` `UserRooms` method are now orphaned — their only
  remaining reference is `resolvers.GetUserRooms`, which was already
  caller-less at base; a `shutdown` artifact is persisted but never broadcast
  (the runner exits before its next snapshot); the QMP client doesn't skip
  async QMP events (pre-existing;
  on the freeze list); `cmd/hostrunner-probe`'s separate QMP reader still has
  no deadlines; standalone `migrate up` on a fresh empty pb_data stack-overflows
  at HEAD (pre-existing, boot-path `serve` migration unaffected).
- 2026-09-01: two adversarial review passes over the branch before push. Test
  gaps they proved by mutation are closed: the persist backstop is now forced
  by a real concurrent insert (`TestPersistFinishedGame_IndexBackstopOnConcurrentInsert`),
  the class pin runs both ways (`rooms.ScraperClasses`), the QMP reset test
  uses a slow server so three commands outrun one window, `RegisterBaseline`
  rejection checks all four registry maps, and `NewReaderForTitle` has a
  no-baseline case. One more discovery: a panic inside `runLive` unwinds
  through the `shutdown` capture and the loop's recover, but the runner stays
  in `Manager.runners` until `Stop` — so join-replay can serve that `shutdown`
  artifact from a dead runner. Pre-existing lifecycle gap; docs now say so,
  and the fix (deregister on loop exit) belongs with the split's runner
  lifecycle work.
