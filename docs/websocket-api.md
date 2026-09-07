# xemu-cartographer WebSocket API — integration guide

> A guide for building an external app (stats tracker, overlay, bot, dashboard) on top of
> the live data this server broadcasts over WebSocket. Written for someone who has **no
> access to the Go source** — everything you need to consume the feed is here.

The server scrapes live state out of one or more running **xemu** (original Xbox emulator)
instances playing **Halo: Combat Evolved**, and streams it to subscribers as JSON over a
single WebSocket. Each xemu instance is called an **instance** / "host" / "pod" and has a
name (e.g. `smoke1`).

---

## TL;DR — the 60-second version

1. Get a PocketBase auth token (log in as a normal user account — see [Auth](#1-authentication)).
2. Open a WebSocket to `ws://<host>:<port>/api/ws?token=<JWT>`.
3. You immediately receive a **`hello`** message listing the instance names that exist.
4. Send `{"type":"join_room","room":"host:<name>:game"}` and
   `{"type":"join_room","room":"host:<name>:event"}` for each instance you care about.
   On join you get a one-time snapshot, then live updates.
5. Every server message looks like `{"type":"scraper","room":"...","payload":{ envelope }}`.
   The inner **envelope** is `{"v":2,"type":"game","instance":"smoke1","seq":42,"tick":...,"ts":"...","data":{ ... }}`.
   Switch on `envelope.type` and read `envelope.data`.

For a **stats app** you almost certainly want exactly three classes:
`game` (roster + cumulative scores/kills/deaths), `event` (kills, deaths, medals as they
happen), and `previous_game` (a finished-match summary + full event log). Everything else is
for overlays/visualizers and can be ignored.

---

## 0. Connection model at a glance

```
                 wss://host/api/ws?token=JWT
   your app  <───────────────────────────────────>  server (one socket)
       │                                                  │
       │  → join_room  host:smoke1:game                   │  (subscribe to "classes")
       │  → join_room  host:smoke1:event                  │
       │                                                  │
       │  ← {type:scraper, payload:{type:hello,    ...}}  │  (on connect)
       │  ← {type:scraper, payload:{type:game,     ...}}  │  (snapshot on join, then live)
       │  ← {type:scraper, payload:{type:event,    ...}}  │  (each kill/death/medal)
       │  ← {type:scraper, payload:{type:previous_game}}  │  (when a match ends)
```

- **One socket, many subscriptions.** You don't open a socket per instance. You open one
  socket and *join rooms* to subscribe to the data you want.
- **A "room" is a (instance, class) pair.** Room names look like `host:<instance>:<class>`,
  e.g. `host:smoke1:game`. Joining a room subscribes you to that one data class for that one
  instance.
- **You only receive classes you've joined.** If you never join `host:smoke1:event`, you
  never see that instance's kill events. Some classes are also only *produced* when at least
  one subscriber is listening, so "subscribe to what you need" is the rule.

---

## 1. Authentication

The socket itself accepts anonymous connections, **but joining any `host:*` room requires a
logged-in user**, and access is further narrowed (see [Access control](#access-control)). So
in practice you need a token.

### Getting a token

This server is a [PocketBase](https://pocketbase.io) app. Authenticate against it and use the
resulting JWT. Easiest with the official JS SDK:

```js
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://localhost:8090'); // same host:port as the API
await pb.collection('users').authWithPassword('me@example.com', 'password');

const token = pb.authStore.token; // <-- the JWT to put in the ?token= query param
```

You can also hit the REST endpoint directly:

```sh
curl -X POST http://localhost:8090/api/collections/users/auth-with-password \
  -H 'Content-Type: application/json' \
  -d '{"identity":"me@example.com","password":"password"}'
# → {"token":"...","record":{...}}
```

The token is passed **only** as the `?token=` query parameter on the WebSocket URL (browsers
can't set custom headers on a WebSocket handshake). The server validates it on connect; an
invalid/expired token is logged and the connection proceeds **as anonymous** (so you'll be
able to connect but get `forbidden` when you try to join host rooms).

### Access control

When you send `join_room` for a host room, the server checks:

| Room | Who may join |
| --- | --- |
| `host:<name>:<class>` (a specific instance) | **Admins** (any instance) **or** a non-admin user whose **gamertag is in that instance's live roster** (re-checked per request). |
| `host:summary` (cross-instance dashboard) | **Admins only.** |

**For a stats app that wants to watch every instance, use an admin account.** A
non-admin/player token can only see the one instance they're currently playing in. If your
join is rejected you'll get an `error` message with code `forbidden`.

> Ask the operator (the person running this server) to either give your service account the
> admin role, or tell you which non-admin scope you have. Roles are managed in PocketBase.

### Host / port

- **Dev:** PocketBase listens on `PUBLIC_PB_PORT` (default **8090**). So
  `ws://localhost:8090/api/ws?token=...`.
- **Prod:** everything is same-origin behind one port. Use `wss://<your-domain>/api/ws?token=...`.

Confirm the exact host/port with the operator.

---

## 2. Message format (two layers)

Every frame is JSON text. There are **two nested layers**:

### Layer 1 — the transport message

```jsonc
{
  "type": "scraper",          // routing tag (see table below)
  "room": "host:smoke1:game", // which room this came from (omitted on some control msgs)
  "payload": { /* Layer 2: the envelope */ }
}
```

`type` values you'll see **from** the server:

| `type` | Meaning |
| --- | --- |
| `scraper` | A data/control envelope. **99% of traffic.** Parse `payload` as the envelope below. |
| `error`   | A problem with something you sent. `payload` is `{"code":"...","message":"..."}`. |

`type` values you **send** to the server are different (`join_room`, etc.) — see
[Client → server](#4-what-you-can-send).

### Layer 2 — the envelope

Inside `payload` (for `type:"scraper"`):

```jsonc
{
  "v": 2,                       // protocol version — see "Versioning"
  "type": "game",               // the CLASS — this is what you switch on
  "instance": "smoke1",         // which xemu instance (empty "" for cross-instance classes)
  "seq": 42,                    // monotonic counter per (instance, class); detect drops/gaps
  "tick": 18342,                // engine tick the data was read at (0 when not in a match)
  "ts": "2026-06-18T20:01:33.123Z", // server send time (RFC 3339)
  "data": { /* class-specific payload */ }
}
```

`payload` and `data` are **already-parsed JSON objects**, not strings — no double
`JSON.parse` needed.

So your read loop is essentially:

```js
ws.onmessage = (e) => {
  const msg = JSON.parse(e.data);
  if (msg.type === 'error') { console.warn('server error', msg.payload); return; }
  if (msg.type !== 'scraper') return;

  const env = msg.payload;          // the envelope
  switch (env.type) {               // the class
    case 'hello':         onHello(env.data); break;
    case 'game':          onGame(env.instance, env.data, env.tick); break;
    case 'event':         onEvent(env.instance, env.data); break;
    case 'previous_game': onPreviousGame(env.instance, env.data); break;
    case 'summary':       onSummary(env.data.hosts); break;
    // tick / objects / debug / scenario / xbox / events — see reference below
  }
};
```

### About `seq`

`seq` is a per-`(instance, class)` counter that starts at `0` and increments by one each
broadcast. Use it to detect dropped or out-of-order messages **within a class stream**. Two
caveats:

- It resets when an instance's runner restarts. The `hello`/runner `started_at` (see below)
  tells you when a restart happened so you know to discard stale `seq` state.
- **Event payloads currently always carry `seq: 0`** (it's a placeholder for live `event`
  envelopes and the `events` reply). For events, order by `tick` / `ts` and arrival order, not
  by `seq`.

### Versioning

`v` is the wire protocol version (currently **`2`**). The `hello` message also reports
`protocol_version`. If you ever see a version you don't recognize, log loudly and consider
refusing to parse rather than silently misreading fields — breaking changes bump this number.

---

## 3. The handshake — `hello`

The **first** message after you connect (sent automatically, before anything else) is a
`hello` envelope. Its `instance` is `""` and `tick`/`seq` are `0`.

```jsonc
{
  "type": "scraper",
  "payload": {
    "v": 2, "type": "hello", "instance": "", "seq": 0, "tick": 0, "ts": "...",
    "data": {
      "protocol_version": 2,
      "server_time": "2026-06-18T20:01:30Z",  // for estimating clock skew
      "classes": ["xbox","scenario","game","game_filtered","tick","objects","debug","summary","previous_game","event","event_filtered"],
      "instances": [
        { "name": "smoke1", "started_at": "2026-06-18T19:55:00Z" },
        { "name": "smoke2", "started_at": "2026-06-18T19:58:12Z" }
      ]
    }
  }
}
```

Use it to:

- **Discover instance names** (`data.instances[].name`) so you know which `host:<name>:...`
  rooms to join. (Non-admins: you may not be told about instances you can't access — rely on
  your own join succeeding/failing.)
- **Detect runner restarts.** Cache each instance's `started_at`; if you reconnect and it
  changed, your cached `seq` and match state for that instance are stale — re-sync.

A fresh `hello` is built on **every** connect, so reconnecting gives you the current world.

---

## 4. What you can send

Send these as JSON text frames. Only `type` (and sometimes `room`/`payload`) matter.

| Send `type` | Fields | Effect |
| --- | --- | --- |
| `join_room` | `room` | Subscribe to a class room. Triggers a one-time snapshot replay of that class's current state, then live updates. |
| `leave_room` | `room` | Unsubscribe. |
| `request_state` | — | Re-send the current snapshot for **every** room you're currently in. Use after a network blip to re-sync without rejoining. |
| `request_events` | `payload: {since_tick?, types?}` | Ask for the recent event backlog of the current match, for each instance you're subscribed to. Returns one **`events`** envelope per instance. |
| `request_probe` | `room`/`payload` | Diagnostics only (raw memory-probe dump). Not needed for stats. |

Examples:

```js
ws.send(JSON.stringify({ type: 'join_room', room: 'host:smoke1:game' }));
ws.send(JSON.stringify({ type: 'join_room', room: 'host:smoke1:event' }));

// Backfill: give me all death/medal events since tick 0 of the current live match
ws.send(JSON.stringify({
  type: 'request_events',
  payload: { since_tick: 0, types: ['death', 'medal'] }
}));
```

Notes on `request_events`:

- Filters are optional. No payload → the whole recent log (newest ~50 events, capped).
- `types` matches the **inner `event_type`** (`death`, `damage`, `medal`, `player_update`,
  `game_update`) — not the envelope's outer `type`.
- The reply only contains events **if the instance is currently Live** (in a match). In
  Idle/Ready phases the event list is always empty — to see a *finished* match's events, read
  the `previous_game` class instead.
- Reply ordering is **oldest-first** (the live `event` stream is naturally chronological too).

### Subscribe to the right room — a common gotcha

Live data is **only broadcast to the per-class rooms** `host:<name>:<class>`. Joining the
bare `host:<name>` room (no class) gives you a one-time multi-class snapshot but **no live
updates**. For a live feed, join each `host:<name>:<class>` you want. To watch instance
`smoke1` for stats:

```
host:smoke1:game     ← roster, scores, cumulative K/D/A, heartbeat
host:smoke1:event    ← live kills/deaths/medals/etc.
host:smoke1:previous_game  (optional) ← snapshot when a match ends
```

---

## 5. Lifecycle / phases

Each instance moves through three phases; the `game` payload's `phase` field tells you which:

| `phase` | Meaning | What flows |
| --- | --- | --- |
| `idle` | At the dashboard / no recognized game loaded. | `xbox` identity, `game` heartbeat (mostly empty). |
| `ready` | In a lobby / pre-game / post-game menu. | `game` with roster + config; `scenario` once a map loads. |
| `live` | Actively in a match (~30 Hz engine). | `game` (cumulative stats), `event` stream, plus `tick`/`objects`/`debug` if subscribed. |

When a match ends, the instance goes `live → ready` and emits one **`previous_game`** envelope
capturing the just-finished match and its complete event log. This is the cleanest hook for
"a game just finished, record it."

---

## 6. The data classes

| Class | Room | Cadence | For a stats app? |
| --- | --- | --- | --- |
| **`game`** | `host:<n>:game` | change-driven, ≤1 Hz heartbeat | **Yes** — roster, scores, cumulative kills/deaths/assists. |
| **`event`** | `host:<n>:event` | as they happen (Live) | **Yes** — kills, deaths, medals, score changes, joins/leaves. |
| `game_filtered` | `host:<n>:game_filtered` | as `game` | Overlays — viewer-safe `game` (same payload shape, dummy/neutral-host players filtered server-side). Stats apps read `game`. |
| `event_filtered` | `host:<n>:event_filtered` | as they happen (Live) | Overlays — viewer-safe deaths-only event stream (positions structurally absent, dummy attributions scrubbed). |
| **`previous_game`** | `host:<n>:previous_game` | once per match end | **Yes** — finished-match snapshot + full event log. |
| **`summary`** | `host:summary` | ≤4 Hz | Optional — one-line status of every instance (admin-only). |
| **`xbox`** | `host:<n>:xbox` | rare | Optional — console/XBE identity, clock. |
| **`scenario`** | `host:<n>:scenario` | once per map load | Optional — static map data (spawns, fog, tag defs). For map views. |
| `tick` | `host:<n>:tick` | ~30 Hz | Overlays only — per-frame player positions/aim/health. Firehose. |
| `objects` | `host:<n>:objects` | ~30 Hz | Map/replay only — world objects + projectiles. Firehose. |
| `debug` | `host:<n>:debug` | ~30 Hz | RE/diagnostics — raw + undecoded biped fields. Firehose. |
| `events` | (reply) | on `request_events` | The backfill reply shape (see §4). |

Below are the **`data` payload shapes** for the stats-relevant classes. Field types are noted
as JSON types; `int16/uint32/...` are just numbers in JSON. A trailing `?` means the field can
be `null` (Go pointer / `omitempty`).

### 6.1 `game` — the core stats class

```jsonc
{
  "phase": "live",                 // "idle" | "ready" | "live"
  "started_at": "2026-06-18T19:55:00Z",
  "last_read_at": "2026-06-18T20:01:33.120Z",
  "engine_tick": 18342,
  "iterations": 91234,             // internal loop counter (freshness)

  "config": {                      // null until a game is loaded
    "gametype": "Slayer",
    "variant_name": "Team Slayer", // optional
    "is_team_game": true,
    "score_limit": 50,
    "time_limit_ticks": 0
  },

  "team_scores": [                 // empty for FFA
    { "team": 0, "score": 31 },
    { "team": 1, "score": 27 }
  ],

  "players": [
    {
      "index": 0,
      "name": "Player1",
      "team": 0,
      "armor_color": 5,            // engine palette index (render a swatch from a per-game table)
      "score": 14,                 // gametype score (CTF caps / Slayer kills / etc.)
      "kills": 14, "deaths": 9, "assists": 3,
      "ctf_score": 0,
      "team_kills": 0, "suicides": 1,
      "kill_streak": 4, "multikill": 2,
      "shots_fired": 410, "shots_hit": 188,
      "is_local": true,            // null if the game can't tell
      "local_index": 0,            // splitscreen slot 0–3, null if remote
      "machine_index": 0,          // system-link machine, null if unknown
      "controller_index": 0        // controller slot on their own machine, null if unknown
    }
    // ...one per player
  ],

  "machines": [                    // connected machines in a system-link lobby
    { "index": 0, "name": "XBOX-A", "is_local": true }
  ],

  "network": {                     // null when no network game
    "countdown": { "active": false, "paused": false, "seconds_to_start": 0 },
    "client":    { "machine_index": 0, "average_ping": 42, "packets_sent": 1200 }
  }
}
```

**This single payload is enough to build a live scoreboard.** It carries the full roster with
cumulative kills/deaths/assists/streaks and team scores, refreshed throughout Ready and Live.
It's emitted on a heartbeat plus whenever something changes.

> Identity note: `name` is the in-game player name. Cross-referencing players to real
> accounts/gamertags is a separate concern handled elsewhere in the app; over this feed you
> get the engine's player name and indices.

### 6.2 `event` — live game events

The `event` class carries **one envelope per event**, in real time during Live. The
`envelope.data` is one of five payload shapes, discriminated by `data.event_type`. All share a
common header:

```jsonc
{
  "seq": 0,                  // placeholder — ignore for events; use tick/order
  "tick": 18342,
  "at": "2026-06-18T20:01:33.118Z",
  "event_type": "death",     // "death" | "damage" | "medal" | "player_update" | "game_update"
  /* ...type-specific fields below... */
}
```

Three small **ref** shapes recur inside events. Identity is *denormalized* onto them — i.e. a
`PlayerRef` records the player's name/team/color **as they were at the moment of the event**,
so an event log alone is enough for analytics even if someone switches teams later.

```jsonc
PlayerRef  = { "index": 2, "name": "Player3", "team": 1, "armor_color": 5 }
ItemRef    = { "spawn_id": 4, "tag": "rocket_launcher" }   // spawn_id omitted for non-spawn items (e.g. dropped weapons)
VehicleRef = { "object_id": 12345, "tag": "warthog" }
```

Treat `PlayerRef.index` as the stable key within a single match.

#### `event_type: "death"`
```jsonc
{
  "event_type": "death",
  "victim":     { /* PlayerRef */ },
  "victim_pos": { "x":1.2, "y":3.4, "z":5.6 },
  "killer":     { /* PlayerRef */ } | null,   // null for non-kill deaths
  "killer_pos": { "x":..,"y":..,"z":.. } | null,
  "cause":  "kill",          // "kill" | "suicide" | "fall" | "betrayal" | "environment" | "unknown"
  "weapon": "pistol",        // "" when unknown
  "team_kill": false,
  "respawn_in_ticks": 150
}
```
This is your **kill feed**. `cause:"kill"` (or `"betrayal"` for same-team) means `killer` is
set; `suicide`/`fall`/`environment` have `killer: null`.

#### `event_type: "damage"`
```jsonc
{
  "event_type": "damage",
  "kind": "hit",             // "hit" | "melee"
  "dealer": { /* PlayerRef */ } | null,   // null for environmental damage
  "dealer_pos": {...} | null,
  "victim": { /* PlayerRef */ }, "victim_pos": {...},
  "amount": 18.5,
  "weapon": "assault_rifle"  // "" when unknown
}
```
High-volume (every HP loss). Usually only needed for detailed analytics; skip for a basic
scoreboard.

#### `event_type: "medal"`
```jsonc
{
  "event_type": "medal",
  "kind": "multikill",       // "multikill" | "kill_streak"
  "player": { /* PlayerRef */ },
  "count": 3                 // e.g. triple kill / 3-streak tier
}
```

#### `event_type: "player_update"`
A union for per-player state changes. `kind` decides which optional fields are present:

| `kind` | Extra fields |
| --- | --- |
| `spawn` | `pos` |
| `score` | `kills?, deaths?, assists?, kill_streak?, multikill?` |
| `powerup_picked_up` / `powerup_expired` | `pos`, `powerup` (`"active_camouflage"`/`"overshield"`) |
| `vehicle_entered` | `pos`, `vehicle`, `seat` (`"driver"`/`"passenger"`/`"gunner"`) |
| `vehicle_exited` | `pos`, `vehicle` |
| `item_picked_up` / `item_dropped` | `pos`, `item` |
| `item_depleted` | `item`, `cause` (`"ammo"`/`"energy"`) |
| `grenade_thrown` | `pos`, `grenade_type` (`"frag"`/`"plasma"`), `remaining?` |
| `player_quit` | — |

```jsonc
{ "event_type": "player_update", "kind": "score", "player": {...},
  "kills": 15, "deaths": 9, "assists": 3, "kill_streak": 5, "multikill": 2 }
```
The `score` kind is a convenient per-player delta if you'd rather react to score changes than
diff successive `game` payloads.

#### `event_type: "game_update"`
Match-level / roster / world changes:

| `kind` | Extra fields |
| --- | --- |
| `player_joined` / `player_left` | `player` |
| `player_team_changed` | `player` (new team), `previous_team?` |
| `team_score` | `team?`, `score?` |
| `game_start` / `game_end` | `map?`, `gametype?` |
| `item_spawned` | `item?`, `pos?` |

```jsonc
{ "event_type": "game_update", "kind": "game_end", "map": "Blood Gulch", "gametype": "Slayer" }
```

### 6.3 `previous_game` — finished-match snapshot

Emitted once when a match ends (and replayed on join). Best hook for persisting completed
games.

```jsonc
{
  "ended_at": "2026-06-18T20:05:00Z",
  "game_uid": "01991f6d3a8e5f0c9b2a7d4e6c1b8a90", // stable per-artifact id — your idempotency key
  "end_reason": "postgame",  // "postgame" | "left_match" | "shutdown" (open set)
  "game": { /* a full `game` payload — final roster + final scores */ } | null,
  "events": [ /* the complete event log for the match, oldest-first */
    { "v":2, "type":"event", "instance":"smoke1", "tick":..., "ts":"...", "data": { /* event */ } }
    // ...
  ],
  "events_truncated": false  // true if the log overflowed the server cap (tail dropped)
}
```

Note `events[]` here are **full envelopes** (each with its own `data` holding the event
payload from §6.2), so you can replay the whole match end-to-end. The log is **complete and
oldest-first** — unlike the `request_events` backfill (a rolling window of the newest ~50),
it holds every event the scraper observed for the match, up to a server cap (currently
10 000); `events_truncated: true` flags the pathological overflow case where the log's
**tail** was dropped.

Field notes:

- **`game_uid`** is minted exactly once when the match ends and is stable across join
  replays and reconnects — **use it as your idempotency key** when persisting, so a
  re-delivered `previous_game` is a no-op. It deduplicates re-deliveries of *this
  instance's* artifact only: on a system-link LAN, each scraping box emits its **own**
  artifact (with its own `game_uid`) for the same match, so cross-box dedupe is your
  side's job (e.g. by (map, gametype, `ended_at`±ε)).
- **`end_reason`** is how the match ended, as this scraper observed it: `postgame` — the
  post-game carnage report was seen (a normal finish); `left_match` — the instance left
  the match without the scraper seeing a post-game screen (e.g. host quit to the menu);
  `shutdown` — the scraper/server shut down mid-match. Treat it as an open set and
  tolerate unknown values. Only `postgame` is a completed match: a `left_match` artifact
  carries whatever roster/scores were last read, so gate rating updates on `end_reason`
  if you only want finished games. A `shutdown` artifact is persisted server-side but
  **never broadcast** — the runner exits before its next snapshot. The only way one
  reaches a client is join-replay (`join_room` / `request_state`) from a runner that
  panicked mid-match and is still registered; treat it like `left_match`.
- **A match is recorded only if the scraper is still attached when it ends.** If the
  emulator crashes (or the instance disappears) mid-game, no `previous_game` is emitted
  for that match at all — there is no crash artifact.
- Servers older than these fields omit `game_uid` / `end_reason` / `events_truncated`
  (and cap `events` differently) — treat all three as optional.

### 6.4 `summary` — cross-instance dashboard (admin-only)

Broadcast to `host:summary`. `instance` is `""`.

```jsonc
{
  "hosts": [
    {
      "instance": "smoke1",
      "phase": "live",
      "title": "Halo: Combat Evolved",
      "xbe_title_name": "Halo",
      "map": "Blood Gulch",
      "gametype": "Slayer",
      "score_summary": "31 — 27",   // compact, may be "" for FFA
      "last_successful_read_at": "2026-06-18T20:01:33Z"
    }
    // ...one per running instance
  ]
}
```
Good for a "what's happening across all pods" overview. Requires an admin token.

### 6.5 `xbox` — machine / title identity (optional)

```jsonc
{
  "title_id": 4294967295, "title": "Halo: Combat Evolved",
  "name": "MyXboxName",                 // console name
  "serial_number": "...", "mac_address": "00:50:...", "video_standard": "NTSC",
  "time_zone": { "bias_minutes": 0, "std_name": "PST", "dlt_name": "PDT" } | null,
  "xbe": { "title_name": "Halo", "version": 123, "game_region": 1, "disk_number": 0, "allowed_media": 0 } | null,
  "kernel": { "system_time": "...", "boot_time": "...", "uptime_seconds": 1234.5 } | null
}
```

### 6.6 `scenario` — static map data (optional, for map views)

One envelope per map load. Contains the map name, difficulty, fog, memory regions, object
types, player spawns, power-item spawns, and a `tag_defs` table (weapon/biped tag metadata
that `tick` references by string). Large but static. You only need this if you're drawing the
map or resolving weapon/biped tag parameters — skip it for pure stats. (Full field list is in
the server's `scenario` payload definition; the shapes are stable per map.)

### 6.7 Firehose classes (`tick`, `objects`, `debug`) — usually skip

These stream at ~30 Hz and exist for live overlays, minimaps, and reverse-engineering — **not
stats**. Only join their rooms if you're building a visualizer, because subscribing makes the
server do the per-frame reads.

- **`tick`** — per-player volatile state each engine tick: `players[]` with
  `pos/vel/aim`, `health`, `shields`, `has_camo`, `frags`/`plasmas`, current weapon slot, an
  `actions` bool bundle (crouching/jumping/firing/…), and a slim `weapons[]`; plus
  `power_items[]`, `ctf_flags[]`, `game_globals`, and per-local `locals[]` (FP weapon, observer
  cam, input).
- **`objects`** — `objects[]` (world objects: vehicles, scenery, dropped weapons, grenades)
  and `projectiles[]`, each with positions/flags.
- **`debug`** — `players[]` with a curated `extended` block, optional `bones[]`,
  `update_queue`, and a `raw` map of still-undecoded fields keyed by hex offset.

If you want positions for a minimap but not at 30 Hz, just throttle on your side after
subscribing to `tick`.

---

## 7. A complete minimal client (browser/Node)

```js
import PocketBase from 'pocketbase';

const API = 'http://localhost:8090';
const pb = new PocketBase(API);
await pb.collection('users').authWithPassword('me@example.com', 'password');

const wsURL = API.replace(/^http/, 'ws') + '/api/ws?token=' + encodeURIComponent(pb.authStore.token);
const ws = new WebSocket(wsURL);

const watch = ['smoke1']; // fill from the hello message if you prefer

ws.onopen = () => console.log('connected');

ws.onmessage = (e) => {
  const msg = JSON.parse(e.data);

  if (msg.type === 'error') { console.warn('server error:', msg.payload); return; }
  if (msg.type !== 'scraper') return;

  const env = msg.payload;             // { v, type, instance, seq, tick, ts, data }
  switch (env.type) {

    case 'hello': {
      // Discover instances and subscribe to game + event for each one we want.
      const names = env.data.instances.map(i => i.name).filter(n => watch.includes(n));
      for (const name of names) {
        ws.send(JSON.stringify({ type: 'join_room', room: `host:${name}:game` }));
        ws.send(JSON.stringify({ type: 'join_room', room: `host:${name}:event` }));
        ws.send(JSON.stringify({ type: 'join_room', room: `host:${name}:previous_game` }));
      }
      break;
    }

    case 'game':
      updateScoreboard(env.instance, env.data);   // env.data.players[], team_scores[], config
      break;

    case 'event':
      if (env.data.event_type === 'death' && env.data.cause === 'kill') {
        recordKill(env.instance, env.data.killer, env.data.victim, env.data.weapon);
      }
      break;

    case 'previous_game':
      persistFinishedMatch(env.instance, env.data); // data.game + data.events[]
      break;
  }
};

ws.onclose = () => {
  // Reconnect with backoff; on reconnect you'll get a fresh `hello` and snapshots.
};
```

---

## 8. Practical notes & gotchas

- **Subscribe to per-class rooms, not the bare `host:<name>`.** Live traffic only goes to
  `host:<name>:<class>`. (§4.)
- **Snapshot-on-join.** When you join a class room you immediately get its current state, then
  live deltas. No separate "get initial state" call needed (but `request_state` re-syncs all
  your rooms if you drop packets).
- **Demand-gated production.** Several classes are only read/emitted while someone is
  subscribed. Your subscription is what turns the data on; if you see nothing, confirm your
  join succeeded (no `forbidden` error) and that the instance is in the expected phase.
- **Reconnects.** On reconnect you get a fresh `hello`. Compare each instance's `started_at`
  to what you cached — if it changed, drop your per-instance `seq`/state and re-sync.
- **`seq` for class streams, not events.** Use `seq` to spot gaps in `game`/`tick`/etc. Events
  always carry `seq: 0` today — order them by `tick`/arrival.
- **Numbers are plain JSON numbers.** The Go side uses sized ints/floats; on the wire they're
  just numbers. `tick` and many counters are unsigned 32-bit — fine for JS `number`.
- **Times are RFC 3339 strings** (`ts`, `at`, `server_time`, `*_at`). Parse with `new Date()`.
- **Origin policy.** If the server sets `WS_ALLOWED_ORIGINS`, your app's origin must be in the
  list (browser clients). In dev all origins are allowed. Ask the operator if a browser
  connection is rejected at the handshake.
- **What you need from the operator:** the host/port, an account token with the right access
  (admin for all instances, or which instance your player token covers), and the instance
  name(s) — though `hello` will list the ones you can see.

---

## 9. Quick reference

**Connect:** `ws(s)://<host>:<port>/api/ws?token=<pocketbase-jwt>`

**Rooms to join (per instance `N`):** `host:N:game`, `host:N:event`, `host:N:previous_game`
(stats) · `host:N:tick`/`:objects`/`:debug` (overlays) · `host:summary` (admin dashboard).

**Send:** `join_room` · `leave_room` · `request_state` · `request_events {since_tick?, types?}`.

**Receive:** `{type:"scraper", room, payload:{ v, type, instance, seq, tick, ts, data }}` —
switch on `payload.type`: `hello` · `game` · `game_filtered` · `event` · `event_filtered` ·
`previous_game` · `summary` · `xbox` · `scenario` · `tick` · `objects` · `debug` · `events`.

**Event types** (`data.event_type` on the `event` class): `death` · `damage` · `medal` ·
`player_update` · `game_update`.
