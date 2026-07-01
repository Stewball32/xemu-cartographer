# Stream assets

The **stream-asset hub** (`/studio/`) is one page that maps every browser-source
stream asset Cartographer ships, lets you **preview** each one live (with no
running game), and **copy its OBS Browser Source URL** with a scoped, read-only
overlay token. This doc is the map of the set and the recipe for adding more.

> Hub URL: **`/studio/`** · Registry: [`sveltekit/src/lib/config/stream-assets.ts`](../sveltekit/src/lib/config/stream-assets.ts)

## What's an "asset"

A stream asset is a render page meant to be loaded as an **OBS Browser Source**.
Two kinds:

- **`overlay`** — transparent background, anchored content, composited over your
  game capture. Lives under `/overlays/[instance]/…` so it inherits the
  chrome-hiding + transparent-background treatment (`/overlays/` is in
  [`HIDDEN_LAYOUT_PATHS`](../sveltekit/src/lib/config/layout.ts)).
- **`scene`** — an opaque, full-frame spectator surface that carries its own
  background (the visualizers). Used as a full scene/source, not an overlay.

Every asset supports two URL modes:

| Mode | URL | Auth | Use |
| ---- | --- | ---- | --- |
| **Mock** | `…?mock=1` | none | Preview / style with an animated sample match — no xemu, no token |
| **Live** | `…?token=<jwt>` | scoped overlay token | The real OBS source for a running container |

The token is minted per **container/instance** and scopes the connection to that
container's `host:<instance>` room (read-only). The **same token works for every
token-scoped asset** — mint once, paste many.

## The set

| Asset | id | Route | Kind | Token | Driven by |
| ----- | -- | ----- | ---- | ----- | --------- |
| **Broadcast scoreboard** | `broadcast-scoreboard` | `/overlays/[instance]/scoreboard/` | overlay | ✓ | `buildScoreboard` + `matchClock` + `statusStrip` |
| **Player cards** | `broadcast-cards` | `/overlays/[instance]/cards/` | overlay | ✓ | `buildScoreboard(game, tick)` |
| **Single card (spotlight)** | _(not a gallery tile)_ | `/overlays/[instance]/card/[slot]/` | overlay | ✓ | `buildScoreboard(game, tick)` |
| Scoreboard (compact) | `scoreboard` | `/overlays/[instance]/` | overlay | ✓ | `buildScoreboard(game, tick)` |
| Match-status strip | `status-strip` | `/overlays/[instance]/status/` | overlay | ✓ | `statusStrip(game, scenario)` |
| **Game timer** | `timer` | `/overlays/[instance]/timer/` | overlay | ✓ | `matchClock(game)` |
| **Kill feed** | `killfeed` | `/overlays/[instance]/killfeed/` | overlay | ✓ | `buildKillFeed(events)` |
| Players (split-screen) | `players` | `/overlays/players/` | overlay | ✗ (user JWT) | `scraperWSV2` direct |
| 2D visualizer | `visualizer-2d` | `/visualizer/[instance]/` | scene | ✓ | `buildVizModel(...)` |
| 3D visualizer | `visualizer-3d` | `/visualizer3d/[instance]/` | scene | ✓ | `buildVizModel(...)` |

**Bold** = the M28 broadcast graphics + the Studio-hub seeds (timer + kill feed).

### Broadcast graphics — themed per game (M28)

The **broadcast scoreboard** and **player cards** are the "hero" surfaces: visually
designed, and **themed per game** via a `?game=ce|h2` switch (default `ce`). One
theme layer ([`components/broadcast/theme.ts`](../sveltekit/src/lib/components/broadcast/theme.ts))
emits `--bc-*` CSS variables — CE reads in UNSC amber/green, H2 in the cool-blue
menu language — so the same live data renders in two visual languages. Both consume
the existing pure builders (no new feed plumbing) and the real player art:

- **Scoreboard** — themed header (gametype · map · `matchClock` · score-to) over
  team columns or an FFA list; each row an armor-chipped player with K/D/A, score,
  and health/shield vitals. Anchors top-center.
- **Player cards** — a bottom strip of per-player cards; each a **Spartan tinted by
  the player's armor colour** ([`CharacterPreview`](../sveltekit/src/lib/components/gamertag/CharacterPreview.svelte)),
  with gamertag, K/D/A, and a big score. **Halo 2 also carries the player's emblem**
  (chest decal + corner badge, real extracted sprites) and renders the Arbiter as an
  Elite. `card/[slot]/` is the single-player spotlight variant.

**CE is live-capable today; the H2 theme is preview-only** until the H2 scraper
(M20) provides the live roster / scores / emblems — the H2 `/studio/` tiles carry
that note. Append `&game=h2` to any broadcast URL to switch theme. See
[M28](milestones/M28-broadcast-graphics.md) for previews + the CE-vs-H2 matrix.

### Game timer (`timer`)

A match clock. Reads `engine_tick` off the `game` payload and divides by the
engine's 30 Hz tick rate (`TICKS_PER_SECOND`) for elapsed time. When the gametype
sets a `time_limit_ticks`, it counts **down** to that limit (clamped at `0:00`);
otherwise it counts **up**. Shows phase (LIVE/SETUP/IDLE), gametype, and the
score-to. Subscribes to only the `game` class.

### Kill feed (`killfeed`)

A rolling stack of recent kills — `killer ✕ victim` with the weapon, betrayals
flagged, suicides/falls called out as self-deaths. It rides the read-only
**`host:<instance>:event` per-class room** (a normal class subscription the
overlay token may make — distinct from the rejected `request_events` backfill).
The pure builder `buildKillFeed(events, max)` projects the WS store's newest-first
event log into render rows; `?max=N` (1–12) caps the visible count.

## Mock mode

All overlays/scenes share one feed abstraction:
[`createOverlayFeed()`](../sveltekit/src/lib/stores/overlay-feed.svelte.ts) wraps
the live `scraperWSV2` singleton **or** swaps in animated sample data
([`overlay-mock.ts`](../sveltekit/src/lib/utils/overlay-mock.ts)) behind one
uniform set of getters, so a page renders identically from either source. Mock
data is a believable 4v4 Team Slayer on Blood Gulch: `mockGame` / `mockTick`
(wobbling bars, creeping score, advancing engine tick) and `mockEvents` (a
scripted kill feed). The hub previews every card via `?mock=1` in a scaled
`<iframe>`.

## How to add an asset

Adding an asset is **a route + a registry entry** — nothing else changes.

1. **Build the render page.**
   - For a transparent overlay, put it under `/overlays/[instance]/<x>/` so it
     inherits chrome-hiding + transparency. Copy an existing `+page.ts` (parses
     `instance` / `token` / `mock` / `scale`) and `+page.svelte`.
   - Drive it from `createOverlayFeed()` and pass the per-class rooms it needs in
     `classes` (e.g. `['game']`, `['game', 'event']`). This is what makes
     `?mock=1` work for free.
   - Keep the projection logic in a **pure builder** in `overlay-view.ts` and
     unit-test it (no socket needed). Add mock fixtures to `overlay-mock.ts` if
     the asset needs a class the mock doesn't emit yet.
   - In `<svelte:head>`, neutralise `html, body` background + `body::before/after`
     and set `overflow: hidden` (copy the block from any overlay page).

2. **Append a `StreamAsset` to `STREAM_ASSETS`** in
   [`stream-assets.ts`](../sveltekit/src/lib/config/stream-assets.ts):

   ```ts
   {
     id: 'my-asset',
     name: 'My asset',
     description: '…',
     icon: SomeLucideIcon,
     kind: 'overlay',            // or 'scene'
     path: (i) => `/overlays/${i}/my-asset/`,
     preview: true,
     aspect: { w: 16, h: 9, label: '1920×1080 · 16:9' },
     obsHint: 'Transparent · anchor top-left'
     // tokenScoped defaults to true; set false only for legacy JWT surfaces.
   }
   ```

   The hub picks it up automatically — gallery card, mock preview, and copy-link.

3. Add a row to the table above. Done.

## Copy-link / token plumbing

The "Copy OBS URL" button mints (or reuses) a scoped overlay token via
`POST /api/overlay-tokens` (gated by `canManageOverlays` — admin/superuser or the
`overlay_manager` role) and builds `…?token=<jwt>`. Token minting for **host
rooms** depends on the WS handshake admitting overlay tokens to `host:*` rooms
(the overlay-scope bypass runs before `RequireAuth` in
[`internal/websocket/handlers/join_room.go`](../internal/websocket/handlers/join_room.go));
without that fix the copy-link URLs connect but get rejected at join.

## OBS setup

1. OBS → Sources → **+** → **Browser**.
2. Paste the copied URL. Set **1920×1080** (overlays self-anchor per their
   `obsHint`; scenes fill the canvas).
3. Leave "Shutdown source when not visible" **off** so it stays connected; tick
   "Refresh browser when scene becomes active" for clean reconnects.
4. Append `&scale=1.5` to enlarge an overlay; `&max=8` to lengthen the kill feed.

## See also

- [M25 — OBS spectator overlays](milestones/M25-obs-overlays.md) — the scoreboard
  + status strip + overlay-token render-page foundation this builds on.
- [M10 — Overlay revamp](milestones/M10-overlay-revamp.md) — the scoped overlay
  token auth.
- [M11/M27 visualizers](milestones/M27-3d-visualizer.md).
