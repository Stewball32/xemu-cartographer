# Splitscreen viewports — per-player on-screen regions

**Status:** CE implemented + unit-tested; engine reads live-verified; the visual
top/bottom convention has one outstanding live pixel-check (see
[Live verification](#live-verification)). H2 documented, not yet implemented.

## What this is

For overlay positioning (nameplates, per-player stats) the scraper reports,
for each player, **which on-screen viewport/region that player occupies** in
splitscreen. The priority case is **2-player splitscreen** — the system-link
setup where each console renders two stacked views.

The viewport is derived from the **engine's own local-player mapping**, not a
heuristic. We do **not** infer a viewport from camera direction or world
position. The engine assigns each local player a fixed splitscreen window via a
per-player index; we read that index and map it to a screen rectangle.

## Engine source (the reliable signal)

Two engine values drive everything:

| Value                  | Meaning                                                                                  |
| ---------------------- | ---------------------------------------------------------------------------------------- |
| `local_player_index`   | Per-player. The splitscreen window slot (`0..3`) for a player **local to this console**. `-1` (0xFFFF as s16) for network/non-local players. **This index *is* the viewport assignment.** |
| `local_player_count`   | Global. The number of local splitscreen players on this console (`0..4`).                 |

### Halo: CE offsets (runtime-verified)

Both are already mapped in the CE scraper and **runtime-verified** against the
stock retail disc:

| Offset constant (haloce)             | Address / field                          | Source                                                  |
| ------------------------------------ | ---------------------------------------- | ------------------------------------------------------- |
| `OffPlrLocalIndex` = `0x02` (s16)    | player datum `+0x02` — local player slot | `player[0]=0, player[1]=1`, RUNTIME-VERIFIED 2026-06-19/21 |
| `AddrPlayersGlobalsPtr` = `0x2FAD20` | `→ players_globals`                       | RUNTIME-VERIFIED 2026-06-21                              |
| `OffPGLocalPlayerCount` = `0x24` (u16) | players_globals `+0x24` — local count   | `local count = 2` in 2-player splitscreen, RUNTIME-VERIFIED 2026-06-21 |

These were confirmed live on a **2-player splitscreen Team Slayer on Prisoner**
match (P0 team 0 / P1 team 1). So the *data the viewport mapping consumes* is
already verified on a real 2-player split; see citations below.

## Viewport → screen-rect mapping

CE's splitscreen layout is fixed by player count. We emit each local player's
**normalized screen rectangle** — origin top-left, every component a fraction in
`[0,1]` (`x,y` = top-left corner, `w,h` = size). It is resolution-independent:
an overlay multiplies the rect by the actual output pixel size to place an
element over the correct region at any resolution.

| `local_count` | Layout              | `local_index` → rect `{x, y, w, h}`                                                                                  |
| ------------- | ------------------- | ------------------------------------------------------------------------------------------------------------------- |
| 1             | Full screen         | `0 → {0, 0, 1, 1}`                                                                                                   |
| **2**         | **Horizontal halves** | **`0 → {0, 0, 1, 0.5}` (TOP)**, **`1 → {0, 0.5, 1, 0.5}` (BOTTOM)**                                                |
| 3             | Quadrants (BR empty) | `0 → {0, 0, 0.5, 0.5}` (TL), `1 → {0.5, 0, 0.5, 0.5}` (TR), `2 → {0, 0.5, 0.5, 0.5}` (BL)                           |
| 4             | Quadrants           | `0 → TL`, `1 → TR`, `2 → BL {0, 0.5, 0.5, 0.5}`, `3 → BR {0.5, 0.5, 0.5, 0.5}`                                       |

**The 2-player split is HORIZONTAL (top/bottom, full width), not left/right.**
This is the retail CE behaviour and is the priority case for this feature.

The mapping is pure and deterministic:
[`internal/scraper/viewport.go`](../internal/scraper/viewport.go) —
`LocalViewport(count, localIndex)` + `AssignLocalViewports(players, count)`.
Unit-tested for all four layouts, out-of-range indices, screen-tiling
invariants, and the 2-player-is-horizontal guard in
[`internal/scraper/viewport_test.go`](../internal/scraper/viewport_test.go).
A matching TS port (`localViewport`) lives in
[`sveltekit/src/lib/types/scraper.ts`](../sveltekit/src/lib/types/scraper.ts)
for overlay consumers.

## Scrape output

Added to the wire (JSON tags):

- **`GameData.local_count`** (`uint16`) — global splitscreen count. Mirrors the
  per-tick `TickPayload.local_count` (already present). `0` outside a local game.
- **`GamePlayer.local_index`** (`*int`, already present) — `null` for
  network/non-local players, `0..3` for locals.
- **`GamePlayer.viewport`** (`*ViewportRect`, new) — the normalized rect for a
  local player; omitted (`null`) for network/non-local players.
- **`TickLocal.viewport`** (`ViewportRect`, new) — the same rect on the per-tick
  per-local list, so the tick stream is self-sufficient for overlay placement.

Populated in CE by
[`internal/scraper/haloce/reader.go`](../internal/scraper/haloce/reader.go)
(`composeGameData` — sets `local_count`, calls `AssignLocalViewports`) and
[`internal/scraper/haloce/reader_locals.go`](../internal/scraper/haloce/reader_locals.go)
(`readLocals` — sets each `TickLocal.viewport`). The per-player `local_index`
was already read at `reader.go` `readGamePlayer` (`OffPlrLocalIndex`).

## Live verification

**Already verified (engine reads):** the inputs to the mapping —
`local_player_index` per player (`0`/`1`) and `local_player_count` (`2`) — were
confirmed live on a **2-player splitscreen** match during the
2026-06-21 CE stock runtime pass (see citations). The scraper reads the exact
offsets that pass verified.

**Outstanding (one narrow check):** pixel-confirm the *visual* convention — that
the player with `local_index = 0` is rendered in the **top** half and
`local_index = 1` in the **bottom** half. This is the well-documented retail CE
behaviour (player 1 on top, player 2 on bottom), encoded in `LocalViewport`, but
it has not been eyeballed against a known controller binding in this project.

To run it on an **isolated test instance** (never a live/production host):

```sh
# Launch xemu VISIBLE/foreground with 2 TOML-bound pads (per the SIGSTKFLT
# learnings — never detached). From a graphical session:
scripts/runtime/xemu-test-harness.sh -n 2 \
  -d "<path>/Halo CE.iso" --hdd "<path>/ce-hdd.qcow2" \
  -q /tmp/xemu-qmp-split.sock --name ce-split-test

# Start a 2-player Split Screen Slayer match (P1 hosts, P2 joins), then attach a
# scraper to /tmp/xemu-qmp-split.sock and confirm:
#   - both players report local_index 0 and 1, local_count == 2
#   - the player visibly in the TOP half has local_index 0; BOTTOM has 1
```

> The two QMP sockets under `containers/xemu/qmp/` (`nexy`, `stew`) are **live
> containers — do not attach to or drive them for this test.** Spin up a
> separate, named test instance with its own socket.

## Halo 2 — offsets to map later (M20)

The H2 live scraper isn't built yet (M20). H2 uses the same local-player model;
record these to wire up when it lands. From
[`halo-offset-mapper/offset-maps/h2-stock.offsets.json`](../../halo-offset-mapper/offset-maps/h2-stock.offsets.json)
(runtime-verified 2026-06-21 on a live 2-player splitscreen Ivory Tower match):

| Value                   | H2 location                                  | Status                                                              |
| ----------------------- | -------------------------------------------- | ------------------------------------------------------------------ |
| Player datum array ptr  | `AddrH2PlayersArrayPtr` = `0x4F01F4`, stride `540` (`0x21C`) | RUNTIME-VERIFIED                                    |
| `local_player_index`    | player datum `+0x1C` (s32)                    | RUNTIME-VERIFIED — `0`/`1` confirmed by diffing the two live players |
| `player_id` (alt)       | player datum `+0x04` (u32) — low byte = local index | RUNTIME-VERIFIED                                            |
| `local_player_count`    | players_globals equivalent                   | **NOT YET MAPPED** — H2 players_globals not located; derive it (or count `local_index >= 0`) when M20 starts |

Once `local_player_index` + a count are read for H2, the **same**
`LocalViewport` / `AssignLocalViewports` mapping applies unchanged (the
splitscreen layout is identical across CE and H2).

## Citations

- CE offsets + verification: `halo-offset-mapper/offset-maps/ce-h1og-default.offsets.json`
  (`player_index` `+0x02` line ~644; `AddrPlayersGlobalsPtr`/local-count line ~1556),
  `halo-offset-mapper/docs/RUNTIME-PASS-2026-06-21-CE-STOCK.md` (local count = 2, 2-player splitscreen Prisoner).
- H2 offsets: `halo-offset-mapper/offset-maps/h2-stock.offsets.json`,
  `halo-offset-mapper/docs/RUNTIME-PASS-2026-06-21-H2-STOCK.md`.
- CE scraper: `internal/scraper/haloce/{offsets.go, reader.go, reader_locals.go, reader_globals.go}`.
- Mapping + types: `internal/scraper/{viewport.go, viewport_test.go, types.go}`.
- Test harness: `scripts/runtime/{padpool.py, xemu-test-harness.sh}`, `docs/XEMU-TEST-SETUP.md`.
