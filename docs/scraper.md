# Scraper

The scraper reads Halo: CE game state out of a running xemu VM's guest memory and turns it into structured snapshot/tick/event payloads. There is **one scraper goroutine per xemu instance**. Everything below is about a single instance's lifecycle — multiple instances run the same logic in parallel, isolated.

> Where it lives: [internal/scraper/](../internal/scraper/) (engine + game plugins) and [internal/scraper/manager/](../internal/scraper/manager/) (lifecycle goroutine).

---

## 1. Lifecycle: when does scraping start and stop?

A scraper for an instance is owned by `manager.Manager`, indexed by instance name. It starts in two ways and stops in three.

### Start triggers

1. **Discovery watcher (auto, the normal case).** When `CONTAINERS_ENABLED=true` and `CONTAINERS_SOCKET_DIR` is set, the watcher in [internal/discovery/watcher.go](../internal/discovery/watcher.go) polls the socket directory every 2s. When a new `*.sock` appears and is dialable, it fires `onAdd(name, sockPath)` → [`Manager.Start`](../internal/scraper/manager/manager.go#L48). This is how scrapers attach to containers spun up via `/api/admin/containers`.
2. **Manual HTTP start.** `POST /api/admin/scraper/start` with `{"name", "sock"}` calls the same `Manager.Start`. Useful for sockets outside the watched directory.

`Manager.Start` does, in order:
- Open `xemu.Instance` against the QMP socket and translate the XBE header GVA so we can read the title ID.
- Run `scraper.Detect` — reads the title ID from the XBE certificate and looks up a registered `GameReader` factory. **If the title ID is unknown, Start fails and no scraper is created.** Currently only Halo: CE (registered in [internal/scraper/haloce/](../internal/scraper/haloce/)) is supported.
- Re-init the xemu instance with the union of detection GVAs and the game's required low GVAs (so all the game's pointer globals are pre-translated).
- Construct a `runner`, store it in `Manager.runners[name]`, and spawn its `loop` goroutine.

### Stop triggers

1. **Discovery watcher detected the socket disappeared.** When the `.sock` file is removed, or fails 3 consecutive dial probes (~6s), the watcher fires `onRemove(name)` → `Manager.Stop`.
2. **Manual HTTP stop.** `POST /api/admin/scraper/stop` (and friends) call `Manager.Stop`.
3. **Server shutdown.** The `OnTerminate` hook cancels the discovery watcher, which then implicitly stops every active runner before the WebSocket hub shuts down.

`Manager.Stop` cancels the runner's context, waits for the loop goroutine to exit (blocks on `<-r.done`), removes it from the map, and closes the xemu instance. It's idempotent — calling Stop on an unknown name is a silent no-op.

**The scraper is "alive" from Start until Stop.** It does not stop on its own. It does not stop when the player goes back to the menu, when the game ends, or when xemu is idle. It just changes what it reads. See §3.

---

## 2. The poll loop

Once a runner is created, [`runner.loop`](../internal/scraper/manager/loop.go#L32) runs forever in its own goroutine. Each iteration:

1. Check ctx — return if cancelled (Stop).
2. Call `reader.ReadGameState()` — a cheap read (a few u8 / pointer derefs) that returns one of `menu` / `pregame` / `in_game` / `postgame` plus the current 30Hz game tick counter.
3. **If the state changed** since the last iteration, emit a snapshot (see §3).
4. **If state == `in_game` AND the game tick advanced** since the last broadcast, do the heavy per-tick read and emit a `tick` envelope plus any detected `event` envelopes (see §4).
5. Sleep until the next iteration. **Cadence depends on state:**
   - `in_game` → 10ms (`inGamePollInterval`) → ~100 Hz polling
   - everything else (menu / pregame / postgame) → 500ms (`idlePollInterval`) → 2 Hz polling

The 30Hz number that shows up around the scraper is **the game's internal tick rate, not the scraper's poll rate.** The 100Hz in-game polling is intentionally faster than 30Hz so we never miss a game-tick advance — the duplicate-tick check on step 4 means only ~1 of every ~3 polls actually does the expensive `ReadTick`; the others just re-read the state header (~5 reads) and bail. The 2Hz idle cadence on the dashboard / pregame / postgame keeps the loop responsive to state transitions without thrashing memory reads while nothing's happening.

---

## 3. What gets read, and when

The scraper has three read tiers, called at different cadences.

### Tier 1: `ReadGameState()` — every iteration

Cheap. Used to decide what to do this iteration. Reads:
- `game_engine_globals` pointer (is the engine running?)
- `main_menu_active` u8 (are we sitting on the main menu?)
- `game_time_globals.{initialized, active, paused, game_time}` (state machine + tick counter)
- `game_can_score` u32 (used to distinguish `in_game` from `postgame`)

Returns `(GameState, tick, error)`. State machine logic is in `determineGameState` in [reader.go](../internal/scraper/haloce/reader.go#L87).

### Tier 2: `ReadSnapshot()` — only on game-state transitions into `pregame` / `in_game` / `postgame`

This is the "heavy one-time read." It captures everything that's static for the lifetime of a match, plus the current scoreboard. Run when transitioning into a state where a snapshot is meaningful — entering pregame lobby, entering in-game, hitting postgame. Skipped on transitions to `menu`.

What it reads:
- **Match config:** map name, gametype, is_team_game, score limit, time limit.
- **Scoreboard:** team scores (when team game), per-player roster (name, team, kills/deaths/assists/etc., is_local + local splitscreen index).
- **Power item spawns:** walks the scenario's item-collection list, filters to tags that have a respawn interval (= power items: rockets, sniper, OS, camo, etc.), records their world coordinates and the initial object ID at game start.
- **Static map data:** game difficulty, all player spawn points, fog parameters, the engine's object-type-definition table, and a diagnostic cache-pointer triplet.

On a transition into `pregame` or `in_game`, the runner also re-creates its `TickState` (per-tick diff tracker) and seeds its `PowerItemTracker` map from the snapshot. This is what stops events from the previous match leaking into the new one (e.g. spurious "kill" diffs against last match's kill counts).

#### Important consequence: snapshot data is frozen between state transitions

`ReadSnapshot` runs **once on entry** to pregame / in_game / postgame and never again until the *next* state change. The loop keeps polling every 500ms (or 10ms in-game) but those polls only read the state-machine header — they don't refresh roster, map, gametype, or anything else from the snapshot.

Concrete implication for **pregame** (lobby): the snapshot fires the instant pregame begins, and from then until the match starts the scraper's view of the lobby is frozen. Players who join, leave, or swap teams mid-pregame are **invisible** to the scraper. Same for a host changing the map or gametype mid-pregame — the scraper won't see the new selection until either:

1. The change drops the lobby back through `menu` and re-enters `pregame` (re-fires snapshot), or
2. The match starts (`pregame → in_game` fires another snapshot and tick reads then run at 30Hz).

The same "frozen until transition" rule applies to `postgame` — the postgame snapshot captures the final state at end-of-match and stays that way until the player exits to menu.

If pregame ever needs to be live (re-read roster as people drop in), it would take a small loop change: either re-snapshot on a timer while in pregame, or do a stripped-down "lobby tick" read at the idle cadence. Today it's strictly one-shot per entry.

### Tier 3: `ReadTick()` — every fresh game tick while `in_game`

This is the 30Hz hot path. Skipped when state isn't `in_game`. Skipped if the tick number hasn't advanced since the last broadcast (deduplicated against ~3 polls per tick).

For every active player slot, it reads:
- **Roster fields** (name, team, is_local) — repeated each tick because seats can be reassigned mid-match.
- **Score counters** (kills, deaths, assists, team kills, suicides, kill streak, multikill, shots fired/hit).
- **Liveness** — is the biped object handle valid? Sets `alive` and (when dead) the respawn timer.
- **Dynamic biped state** when alive — position, velocity, aim, zoom, crouch scale, health, shields, camo / overshield flags, frags / plasmas, selected weapon slot, action bitfield (crouching/jumping/firing/melee/grenade/flashlight/use), melee state, parent vehicle handle, four weapon slots.
- **Per-weapon detail** for each non-empty slot — tag name, ammo or charge, "extended" weapon-object state (heat, reload, world position when dropped), and cached static weapon-tag-data (zoom levels, autoaim, magnetism, animation array).
- **Extended biped state** (~50 diagnostic fields: legs/aim rotations, angular velocity, aim-assist sphere, scale, animation IDs, damage countdowns, airborne/landing state, etc.).
- **Bones** — 19 model-node positions for skeletal pose.
- **Update-queue slot** — per-player input replication state (buttons, sticks, desired yaw/pitch). Read for every player including dead/remote.
- **Biped tag-data** — static per-tag fields (autoaim pill radius, flags), cached per tag index.
- **Damage table** — last 4 damage events on the biped. Used for kill attribution.

Beyond per-player, each tick also reads:
- **Team scores** (when team game).
- **Power item live status** — for every spawn from the snapshot, looks up its current state: `held` (in some player's weapon slot), `world` (spawned but uncarried; reports world position), or `respawning` (decrementing a per-spawn timer).
- **Game globals** — map_loaded, active, double-speed flag, loading state, precache progress, difficulty, RNG seed.
- **Local-player subsystems** — for each splitscreen slot 0..3: first-person weapon, observer cam, input abstraction, gamepad raw, UI globals (color, button config, sensitivity, profile index), player_control struct, look rates.
- **All non-garbage world objects** — generic per-object data (id, tag, position, angular velocity, owner, parent).
- **Network state** — client (machine index, pings, packets), server (countdown), inline game data, network machines, network players.
- **Data queue header** — engine's input replication queue (tick, RNG, player count).
- **CTF flags** (when applicable) — per-flag position + carrier + status.
- **Projectiles** — for every projectile object: position, flags, action, detonation timer, target, deceleration, rotation axis, etc.

After all that's read into a `TickResult`, `DetectEvents` runs (§4) before broadcasting.

---

## 4. Event detection

[`DetectEvents`](../internal/scraper/haloce/events.go) compares the just-read `TickResult` against the per-runner `TickState` (diff buffer from the previous tick) and emits zero or more `event` envelopes. Detected events include: `kill` (with team_kill subtype), `death`, `spawn`, `damage`, `melee`, `grenade_thrown`, `item_picked_up`, `item_dropped`, `item_spawned`, `item_depleted`, `powerup_picked_up`, `powerup_expired`, `multikill`, `kill_streak`, `score`, `vehicle_entered`, `vehicle_exited`, `player_quit`, `game_start`, `game_end`. Kill attribution falls back to the biped damage table when kill counters haven't ticked yet.

Then `TickState` is updated to "this tick's values" so the next tick can diff against it.

---

## 5. State summary

| State | Poll cadence | Reads while in this state | Snapshot on entry? | Live data refresh? |
| ----- | ------------ | ------------------------- | ------------------ | ------------------ |
| `menu` | 500ms | `ReadGameState` only | no | n/a |
| `pregame` | 500ms | `ReadGameState` only | yes (resets TickState) | **no** — roster/map/gametype frozen at snapshot |
| `in_game` | 10ms | `ReadGameState` + (on fresh tick) full `ReadTick` + `DetectEvents` | yes (resets TickState) | yes — every game tick (~30Hz) |
| `postgame` | 500ms | `ReadGameState` only | yes | **no** — final scoreboard frozen at snapshot |

A scraper attached to a VM sitting on the dashboard just polls the state-machine header every 500ms. It costs ~nothing until a game enters pregame; then it fires one snapshot and goes back to 500ms header-only polling. Once the match starts it ramps to 10ms polling and full per-tick reads for the duration of the match; then back down at the end-of-match transition. **Pregame and postgame are essentially "snapshot, then idle" states** — the loop is alive but the captured data doesn't update until the next state transition.

---

## 6. Caches that survive the loop

These are populated lazily by reader code and reused across ticks (sometimes for the runner's whole lifetime):

- **`tagNameCache`** — tag index → tag name string. Filled the first time each tag is seen.
- **`weaponTagDataCache`** — static weapon-tag-data per tag index. Map-static, never invalidated.
- **`bipedTagCache`** — static biped-tag-data per tag index. Same lifetime.
- **`tagInstBase`, `ohdBase`** — pointer bases dereferenced once at first-tick, cached as plain fields on `Reader`.

These caches are NOT safe for concurrent reads, which is why the inspect HTTP handler reads only the manager's separately-locked snapshot/tick/state copies, never `r.reader` directly.
