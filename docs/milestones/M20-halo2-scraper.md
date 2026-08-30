# Milestone 20 — Halo 2 scraper (with known caveats)

> Demoted to last per user direction. Validates the registry abstraction holds for a non-CE Halo title, and consumes M19's offset validation from day one.

- Port `internal/scraper/halo2/*` preserving **every** `UNVERIFIED` comment.
- Known broken areas (each gets its own follow-up task):
  - Event buffer (`GVAEventCount` always reads 0) — may not exist in xemu's layout; re-derive offsets or find an alternative data source.
  - Objects datum array → real `Alive / Health / Shields / Vehicle` values (currently hardcoded stubs).
  - Team index / primary color / gametype (`SessOffTeamIndex`, `SessOffPrimaryColor`, `GRGVarGameTypeOff`).
- Wire into M19 offset validation — H2 fields enter the system already gated by sanity checks.
- Wire into M13/M15/M16/M18 — game records, stats, tournaments, ratings all become game-type aware including Halo 2.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-08-29: **The "H2 offsets" general blocker removed** (branch
  `update/halo2offsets`). Two halves, per halo-offset-mapper
  `docs/h2-slim-offsets-2026-08-13.md`:
  1. **Wire**: the `game`-class envelope now carries `game_state` — the
     reader's lossless menu/pregame/in_game/postgame — alongside the 3-value
     `phase` that collapsed lobby+postgame into `ready`. Additive; CE lights
     up automatically, overlays can finally tell a lobby from a carnage
     report.
  2. **Offsets import + reader wiring**: 13 runtime-verified entries from the
     mapper's h2-stock map (K/D semantics 2026-07-11; gametype enum induced
     live; system-link machine layer) into `h2-baseline.json` + the versioned
     struct. The reader now emits real per-player kills (u16@0x519500+2i) /
     deaths (u32@0x51975C+4i), betrayals→team_kills (LOCAL-ONLY caveat),
     gametype (1 ctf / 2 slayer / 3 oddball / 4 king), per-player
     machine_index + is_local/local_index (POV membership), machines[] from
     the MAC array + page-guarded table names, universal map names via the
     scenario-path pool read (DLC included), and — on builds that map it —
     the full 4-state lifecycle from `AddrH2GamePhase` (Slim 0x527334,
     full-cycle validated; stock carries a 0 "unmapped" sentinel and keeps
     the array inference).
  Honestly still absent (no offsets exist on ANY build): assists, score,
  kill-streak, suicides, team scores; events. The H2 Slim CLIENT/SERVER
  builds need their remaining absolute addresses recovered (mechanical
  find-then-sweep method documented) — once mapped, their sets import AT
  RUNTIME via Organizer → Offsets (M29), no deploy.
