# Milestone 20 — Halo 2 scraper (with known caveats)

> Demoted to last per user direction. Validates the registry abstraction holds for a non-CE Halo title, and consumes M19's offset validation from day one.

- Port `internal/scraper/halo2/*` preserving **every** `UNVERIFIED` comment.
- Known broken areas (each gets its own follow-up task):
  - Event buffer (`GVAEventCount` always reads 0) — may not exist in xemu's layout; re-derive offsets or find an alternative data source.
  - Objects datum array → real `Alive / Health / Shields / Vehicle` values (currently hardcoded stubs).
  - Team index / primary color / gametype (`SessOffTeamIndex`, `SessOffPrimaryColor`, `GRGVarGameTypeOff`).
- Wire into M19 offset validation — H2 fields enter the system already gated by sanity checks.
- Wire into M13/M15/M16/M18 — game records, stats, tournaments, ratings all become game-type aware including Halo 2.
