# Milestone 10 — Overlay revamp + new browser sources

> Current overlay at [sveltekit/src/routes/overlays/players/+page.svelte](../sveltekit/src/routes/overlays/players/+page.svelte) is keyed to `firstGameData` / `firstTick` (the legacy single-instance accessor) and shows local players only. Rebuild around the M5 multi-instance model where overlays bind to a specific machine's POV — sometimes the host container, sometimes a guest the host is connected to — and add new overlay surfaces beyond the current player HUD.

## 10a. POV-bound overlay routing

Route shape: `/overlays/<surface>/<machine_name>/` (surface first — groups by overlay type, e.g. `/overlays/scoreboard-detailed/halo-host-1/`). The overlay subscribes to the `host:<container>` room whose roster contains `<machine_name>` (lookup via the M9a aggregator extension). Players' POV is then rendered relative to that machine's seat in the local players list. Replace `firstGameData` / `firstTick` with this lookup pattern; deprecate the legacy accessors.

## 10b. Scoreboard surfaces

Two browser sources at `/overlays/scoreboard-simple/<machine>/` and `/overlays/scoreboard-detailed/<machine>/`. Simple = team scores + match clock. Detailed = full per-player K/D/A, current weapons, alive/dead state.

## 10c. Event popup overlay

`/overlays/events/<machine>/`. Renders animated card-style popups for kill chains (multi-kills, kill streaks), CTF captures, oddball/hill events, juggernaut transitions. Likely needs an animation library beyond raw CSS — candidates: Svelte's built-in `transition` + `motion`, or a small library like `@svelte-motion`. Decide during 10c.

## 10d. Dummy-player / neutral-host filter

In modded Halo: CE matches with a neutral host, the host container spawns a dummy player out-of-bounds that never participates. Without filtering it shows up in the overlay, the scoreboard, and (later) the stats. Implement a filter at the data layer in [internal/scraper/manager/](../internal/scraper/manager/) (or a sibling helper) so the same filter applies to overlays, minimaps (M11), and stats (M15). Three configuration sources:

- Per-container flag `is_neutral_host` (likely added to the container record managed by [internal/podman/](../internal/podman/) or as a sidecar config; defaults false).
- A global allowlist of "always-dummy" gamertags (configurable via PB schema in 10d, e.g. `dummy_gamertags` collection).
- A per-game manual override accessible from the M15 stats UI for after-the-fact correction.

The filter takes a roster + the container's neutral-host flag and returns the cleaned roster. Overlays/minimaps consume the cleaned roster; raw debug page (M6c) still shows the unfiltered view for diagnostics.

## 10e. POV correctness pass

Today the overlay assumes the rendering machine *is* the local one. After 10a's refactor, the overlay can be POV-bound to any machine in any container's roster — confirm tag names, weapon slots, and stat indices are correct for the targeted machine, not the host. Likely surfaces edge cases in the Halo: CE reader; file follow-ups for M19 if found.

Smoke test (matches M5's 4-instance pattern): start 4 containers (one flagged neutral-host) in a system-link match, open `/overlays/scoreboard-detailed/<machine_a>/` and `/overlays/events/<machine_b>/` in separate OBS Browser Sources, run a 5-minute match. Verify POV correctness, animation timing, OBS transparency, and that the neutral-host's dummy player is absent from both overlays. Re-validate the existing players overlay through the new routing.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-18: First increment — the **testable data foundation** (10d + the 10a lookup), implemented during the autonomous overnight run. The live overlay surfaces (10a routing pages, 10b scoreboards, 10c event popups, 10e POV pass) are **deferred** — they need live multi-instance scraper data + OBS to verify, and that verification can't run in CI / this environment. What landed:
  - **10d — dummy-player / neutral-host filter (data layer).** New `internal/scraper/roster` package with `FilterRoster(players, Config{IsNeutralHost, DummyGamertags})` — a pure, unit-tested helper that drops (a) a neutral host's local out-of-bounds dummy (`IsLocal` players when `IsNeutralHost`) and (b) any player whose sanitized name is in the global allowlist. Never mutates its input; preserves nil. `BuildDummySet` turns raw allowlist rows into the sanitized lookup set. This is the single cleaning step the spec wants shared across overlays / minimaps (M11) / stats (M15); the raw debug page keeps the unfiltered roster.
  - **10d config sources.** Two of the three: per-container `is_neutral_host` bool on the `containers` collection (defaults false), and the global `dummy_gamertags` collection (admin-gated CRUD via `hasAdminRole`, registered through `identity.go` phase 2 like `capture_policies`). The per-game manual override is explicitly M15's job.
  - **10a — machine→container lookup.** No new code needed: M9's `scraperiface.MatchContainer(Membership(), [machineName])` already resolves a machine name to its container (the membership `Identities` include machine names). The overlay routing that consumes it is part of the deferred UI increment.
  - **Deferred (needs live data + OBS, with an open decision):** wiring `FilterRoster` into the manager's broadcast path (needs per-container `is_neutral_host` lookup + caching, and consumers that don't exist yet); the overlay UI surfaces; the animation-library decision for 10c; the 10e POV-correctness reader audit. **Open question for Stewart:** overlay auth — the existing overlay (`/overlays/players/`) connects through the authed `scraperWSV2` store, and M09 9c narrowed `host:<name>` to admin-or-roster-member. For OBS browser sources run by a tournament operator (admin) this is fine, but if overlays should be runnable by an unauthenticated/token-only OBS instance, the room-auth model needs a deliberate design pass.
