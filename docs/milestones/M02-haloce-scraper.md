# Milestone 2 — Halo: CE scraper

**Status:** Ported. End-to-end smoke-tested against a native xemu running Halo: CE in splitscreen — `POST /api/admin/scraper/start` auto-detects title `0x4D530004`, runner streams snapshot/tick/event envelopes at exactly 30Hz to WebSocket clients in the new `overlay` room, both local players' positions / aim vectors / health+shields / weapons (incl. ammo, energy charge, energy-vs-ballistic flag, full tag names like `weapons\sniper rifle\sniper rifle`) / camo + overshield bools all render correctly. Stop is idempotent (POST `/stop/{name}` → 204). One Yama gotcha to remember: `kernel.yama.ptrace_scope=0` (or `setcap cap_sys_ptrace=eip` on the dev binary) is required for the server to read `/proc/<xemu-pid>/mem`. The legacy file-cap workaround documented in M1 still applies if your xemu install carries `CAP_NET_*` for pcap netplay.

What landed:

- **2a — full offset audit.** Reconciled all 515 hex constants from `atlas/HaloCaster/HaloCE/halocaster.py` against the 128-offset legacy Go table. Active read-path constants live in [internal/scraper/haloce/offsets.go](../internal/scraper/haloce/offsets.go); every other corroborated offset organised by struct in [internal/scraper/haloce/offsets_reference.go](../internal/scraper/haloce/offsets_reference.go). Each constant carries a `// halocaster.py:NNN` origin tag. All marked `unverified` until M19's runtime sanity-check pass.
- **2b — scraper code ported.** [reader.go](../internal/scraper/haloce/reader.go), [events.go](../internal/scraper/haloce/events.go) (19 event types via stat-diff + damage-table fallback), [game.go](../internal/scraper/haloce/game.go) (`init()` registers Halo: CE with `scraper.Lookup`), [xboxname.go](../internal/scraper/haloce/xboxname.go).
- **2c — WS wiring.** New [internal/scraper/manager](../internal/scraper/manager) package owns per-instance lifecycle (Start / Stop / List) and the 30Hz tick goroutine. Decision: **wrap, not extend** — every broadcast becomes `Message{Type:"scraper", Room:"overlay", Payload:<envelope-json>}` so the wire schema stays uniform across all rooms ([loop.go](../internal/scraper/manager/loop.go)). New `overlay` room with `RequireAuth` ([rooms/overlay.go](../internal/websocket/rooms/overlay.go)). New `Scraper` field on `guards.Services` backed by `internal/guards/interfaces/scraper/` (one-method-per-file).
- **2d — admin routes + main.go wiring.** `GET /api/admin/scraper`, `POST /api/admin/scraper/start`, `POST /api/admin/scraper/stop/{name}` ([routes/scraper](../internal/pocketbase/routes/scraper)), all gated by `RequireAuth + RequireAdmin`. `cmd/server/main.go` builds the `Services` skeleton early so the scraper manager gets a stable `*Services` pointer; subsystems mutate fields as they come up. Blank import `_ "internal/scraper/haloce"` triggers the title-ID registration.

## M2 follow-ups (deferred)

- ~~**Snapshot replay for late joiners.**~~ Resolved during M4 with option (a): each `runner` caches the most-recent `websocket.Message` bytes for its snapshot envelope; the `join_room` handler replays them via the new `SendRaw` event capability when a client subscribes to `overlay`. See `internal/scraper/manager/{runner.go,loop.go,manager.go}`, `internal/guards/interfaces/scraper/snapshot.go`, `internal/websocket/handlers/join_room.go`.
- **Investigate `power_items: null` in tick payloads.** During the smoke test the initial snapshot's `PowerItemSpawns` came back empty (likely the scenario wasn't fully loaded when the scraper started, since power-item resolution depends on world-object scanning). Worth re-running the smoke test with start-after-match-ready and confirming spawns populate; if they still don't, that's a Halo offset divergence to chase during M19.

## 2a. Offset audit (prerequisite)

The legacy Go offset table has 128 hex constants; HaloCaster's `HaloCE/halocaster.py` has 515 scattered across 2587 lines. Before trusting the legacy table as complete:

1. Read `atlas/HaloCaster/HaloCE/halocaster.py` end-to-end, extracting every memory-offset-like constant with surrounding context (what struct, what field, what read type).
2. Diff the extracted set against `atlas/xemu-cartographer-legacy/internal/scraper/haloce/offsets.go`.
3. Categorize the deltas:
   - Genuinely missing offsets the legacy reader never used → port them.
   - Non-offsets (struct sizes, magic values, indexing math) → document in comments, don't port.
   - Offsets that exist in both but differ in value → investigate (xemu vs. real-Xbox divergence is plausible).
4. Produce a reconciled `internal/scraper/haloce/offsets.go` in the new repo, each offset annotated with its HaloCaster origin (file + line) and verification status.
5. Flag offsets needing runtime verification for Milestone 19's sanity-check work.

## 2b. Port the scraper code

- Port `internal/scraper/haloce/{reader.go,events.go,game.go}` using the reconciled offset table.
- If the audit surfaced offsets for fields the legacy reader never consumed, extend `reader.go` to populate them.

## 2c. Wire to the template's WebSocket Hub

Adapt the legacy tick-loop to the template's `internal/websocket/` Hub. Decide during this milestone:

- **(a)** Wrap the legacy `Envelope{Type, Instance, Tick, Payload}` inside the template's existing `message.Message`, or
- **(b)** Extend the template's `message.Message` to carry the legacy envelope directly.

## 2d. Smoke test

Halo: CE match in manually-started xemu → snapshots + ticks + events flow to `/api/ws` clients. Fields added from the audit render plausibly in a debug overlay.
