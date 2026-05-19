# Milestone 19 — Robustness + offset validation

> Split from the original M8 (Robustness + Discord + auth). The Discord pieces (commands, channel posting) are subsumed by M17; the auth-wrapping work is mostly addressed by M6a + M7c + M8 + M9c; multi-user UX is covered by M15/M16. What remains is the "make silent bad data become loud errors" work, plus general operational hardening.

- **19a. Runtime offset sanity checks.** Apply to both Halo: CE and Halo 2 (whenever it lands). Base-HVA range checks, magic-value probes, plausibility bounds on read values. Loud error (log + WS notification + admin debug page badge) on sanity-check fail.
- **19b. Field-level validation.** Use the M6c debug-page audit's "looks plausible / clearly broken" annotations as input — promote validated fields out of `unverified` status, file remaining `unverified` fields as offset-investigation tasks.
- **19c. PB queue + scraper resilience.** Polish M13d's queue logic; add metrics; ensure scraper restarts cleanly after PB outages.

Smoke test: deliberately corrupt an offset in the Halo: CE table → loud error fires within one read; recover by reverting; confirm no silent bad-data writes to PB.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-05-18: queued from M6c audit pass against live `debug-host` (Halo: CE multiplayer, `levels\test\downrush\downrush`, four power-ups). Two confirmed-broken Halo: CE offset families that need re-derivation against the Xbox build (halocaster.py was the Halo PC target):
  - `readObjectTypes` ([internal/scraper/haloce/reader_static.go:101](../../internal/scraper/haloce/reader_static.go)) — `RefAddrObjectTypeDefRangeLo = 0x1FC0D0` / `RefAddrObjectTypeDefRangeHi = 0x1FCBA4` / `RefAddrObjectTypeDefArray = 0x1FCB78` ([internal/scraper/haloce/offsets.go:498/502/503](../../internal/scraper/haloce/offsets.go), halocaster.py:344/734/742). Returns empty slice on live wire envelope.
  - `readPowerSpawnScenarios` ([internal/scraper/haloce/reader.go:1124](../../internal/scraper/haloce/reader.go)) — `scenarioBase` deref works (player_spawns are populated through the same pointer chain), so failure is downstream. Candidate gates: `OffScenarioItemCount = 900` / `OffScenarioItemFirst = 904` ([offsets.go:426-427](../../internal/scraper/haloce/offsets.go), halocaster.py:631-632) for the item-array bounds, or `OffTagRespawnIntervalOff = 0x14` / `OffTagRespawnInterval = 0x0C` ([offsets.go:436-437](../../internal/scraper/haloce/offsets.go), halocaster.py:655) for the per-item respawn interval read — `interval<=0` in `readSpawnInterval` ([reader.go:1257](../../internal/scraper/haloce/reader.go)) silently drops items.
  - Note: the M6 Log's 2026-05-18 "Fix C (scenario.map) deferred" entry was reassessed on 2026-05-19 against a 5-container fleet and confirmed to be a real offset bug after all — `AddrMultiplayerMapName = 0x2E37CD` is a halocaster.py:1892 port targeting Halo PC, not Xbox. Closed by replacing both call sites with a tag-instance walk that matches the scenario tag's `data_ptr` to the global scenario pointer (see M6 Log 2026-05-19). Strikes the morning-of-2026-05-18 "probably not an offset bug" claim from this note.
  - Approach when picked up: add diagnostic logging on each silent-nil gate in the three readers; run against `debug-host` mid-match; capture which gate trips. Then either re-derive the address (atlas/HaloCaster offers cross-references — re-verify against current xemu memory, do not trust blindly) or replace the read with a different derivation path (e.g. walk tags by name string rather than index range).
