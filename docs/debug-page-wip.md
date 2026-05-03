# Admin debug page — WIP notes

Snapshot of where the debug page rewrite + scraper diagnostic work is paused.
Branch: `wip/scrape-coverage-and-debug-page`.

## What landed

- **Frontend**: `/admin/debug/[name]/` rewritten with Skeleton Tabs (Overview /
  Snapshot / Tick / Events / Probe / Raw JSON), a global "Show all data" toggle
  (persisted to localStorage), and per-section sub-components in
  [`sveltekit/src/lib/components/debug/`](../sveltekit/src/lib/components/debug/).
- **Overview tab**: state badge, gametype/team/FFA badges, engine-tick counter
  (driven by WS `tickNumbers` so it updates ~30 Hz, not the 3 s HTTP cadence),
  match config, score block (team or FFA leaderboard), single aligned roster
  table with live HP/Sh from tick + per-player Score/K/D/A/KS, and a
  "State inputs (diagnostic)" KV card.
- **Probe tab**: dumps every candidate address the Halo: CE plugin reads for
  gametype detection, team scores, score limits, and per-player scores. Backed
  by `BuildScoreProbe()` on the `GameReader` interface; impl in
  [`internal/scraper/haloce/reader_probe.go`](../internal/scraper/haloce/reader_probe.go).
- **Backend diagnostic surfaces**: `StateInputs` and `ScoreProbe` types added
  to `internal/scraper`, exposed through the `GameReader` interface, cached on
  the runner and copied into `InspectState` so the inspect endpoint serves
  them directly.
- **Per-gametype per-player Score**: `SnapshotPlayer.Score` populated by
  `fillPlayerScores` using either `ctf_score` (CTF) or the Slayer / Oddball /
  King / Race per-player table at `score_base + 64 + 4*idx`.

## Open issues — pick up here

1. **Gametype detection still wrong.** `readGametypeID` was switched from the
   `AddrVariant` byte (which gave `1`/"ctf" in a Slayer match — coincidence) to
   `*AddrGameEngineGlobalsPtr + OffGEGGametype (0x04)` per legacy halocaster.py.
   In a live in-game session this reads `0` ("none"), so either the offset is
   wrong on this build or the pointer doesn't carry that field where legacy
   thought. **Action**: open the Probe tab in-game, read the
   `gametype_candidates` block, and find the value that matches the actual
   running gametype (e.g. `2` for slayer, `1` for ctf). Likely candidates:
     - one of `ge_plus_XX_u32` / `ge_plus_XX_u8` at a different offset
     - `global_variant_at_2f90a8_u32`
     - `game_variant_global_at_2fab60_u32`
     - the `ge_globals_first_64_bytes_hex` dump may show it visually
   Once identified, update `readGametypeID` (and probably the `OffGEGGametype`
   constant) in [`internal/scraper/haloce/reader.go`](../internal/scraper/haloce/reader.go).

2. **Score-limit reads wrong without correct gametype.** `readScoreLimit`
   switches on the same broken gametype ID, so the score limit shows `none`
   instead of `50` in a 50-kill Slayer match. **Action**: fixing #1 will fix
   this automatically.

3. **Per-player score is empirically right today.** The Score column shows
   correct Slayer values (and went down on suicide) because we observed the
   per-player score lives at the static-player struct offset 0xC4
   (`OffPlrCTFScore`) for Slayer too. Once #1 is fixed, the gametype-table
   path in `fillPlayerScores` should be cross-checked against the static-
   struct value via the Probe tab's `per_player_static_struct` and
   `per_player_score_tables` blocks. They should agree for Slayer.

4. **Score updates lag ~3 s.** Snapshot is only re-broadcast over WebSocket
   on `GameState` transitions (loop.go:84-99). The per-tick `ReadLobby` call
   refreshes the cached snapshot but doesn't broadcast — so the debug page
   only sees fresh scores when its 3 s HTTP poll fires. **Action**: in
   [`internal/scraper/manager/loop.go`](../internal/scraper/manager/loop.go),
   add a periodic snapshot broadcast on a fresh tick (every N ticks, e.g.
   N=15 for ~2 Hz) so live scores reach the overlay/debug pages without the
   HTTP poll. Be careful not to spam — N=1 would send 30 snapshots/sec.

5. **Snapshot / Tick / Events tabs not yet iterated.** Only the Overview tab
   has had user feedback. The other tabs render correctly but haven't been
   refined. The plan in
   [`/home/stew/.claude/plans/i-m-working-on-scraping-twinkling-fox.md`](../../.claude/plans/i-m-working-on-scraping-twinkling-fox.md)
   (local file, not in repo) outlines the master-detail layout for Tick →
   Players, sub-tabs for Network/Objects/Projectiles/CTF/Locals/Misc, and the
   Snapshot accordion structure. Continue from there once the score/gametype
   issues are resolved.

6. **Lobby state still classified as `menu` (Part A of the plan).** The
   multiplayer setup screen (where players join, pick teams, choose gametype
   before pressing Start) shows as `menu` and the loop skips snapshot reads
   there. **Action**: drive xemu through main menu → Multiplayer → System Link
   → MP setup with the Overview tab open; record the `state_inputs` tuple at
   each step; identify the combination unique to MP setup; add
   `GameStateLobby` in `internal/scraper/types.go`, extend `determineGameState`
   in `reader.go`, and handle `GameStateLobby` in the `loop.go` switch (lines
   84 + 152) the same way `pregame` is handled.

## How to resume on a fresh machine

```sh
git fetch origin
git checkout wip/scrape-coverage-and-debug-page
git pull
task dev
```

Then open `http://localhost:5173/admin/debug/<instance>/` while a Halo: CE
session is running under that instance's QMP socket, log in as a superuser,
and click the Probe tab.
