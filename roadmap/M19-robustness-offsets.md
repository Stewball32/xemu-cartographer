# Milestone 19 — Robustness + offset validation

> Split from the original M8 (Robustness + Discord + auth). The Discord pieces (commands, channel posting) are subsumed by M17; the auth-wrapping work is mostly addressed by M6a + M7c + M8 + M9c; multi-user UX is covered by M15/M16. What remains is the "make silent bad data become loud errors" work, plus general operational hardening.

- **19a. Runtime offset sanity checks.** Apply to both Halo: CE and Halo 2 (whenever it lands). Base-HVA range checks, magic-value probes, plausibility bounds on read values. Loud error (log + WS notification + admin debug page badge) on sanity-check fail.
- **19b. Field-level validation.** Use the M6c debug-page audit's "looks plausible / clearly broken" annotations as input — promote validated fields out of `unverified` status, file remaining `unverified` fields as offset-investigation tasks.
- **19c. PB queue + scraper resilience.** Polish M13d's queue logic; add metrics; ensure scraper restarts cleanly after PB outages.

Smoke test: deliberately corrupt an offset in the Halo: CE table → loud error fires within one read; recover by reverting; confirm no silent bad-data writes to PB.
