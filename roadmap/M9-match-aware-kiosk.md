# Milestone 9 — Match-aware kiosk view

> A logged-in player should see the kiosk for the container they're playing in (and only that one), automatically. The existing per-container kiosk at [sveltekit/src/routes/containers/[name]/+page.svelte](../sveltekit/src/routes/containers/[name]/+page.svelte) is admin-gated and assumes you know the container name. Replace that mental model with "log in → see your match", driven by gamertag-to-machine-name detection inside the running scraper data.

## 9a. Gamertag → machine-name detection

Each scraper runner already exposes the local Xbox machine name and the network player roster (machines + gamertags) via the M5 `instanceCache`. Extend the runner to publish a `(container, machines[], gamertags[])` membership view — likely a new field on the `host:all` summary aggregator in [internal/scraper/manager/aggregator.go](../internal/scraper/manager/aggregator.go). The same view feeds M10's overlay routing.

## 9b. Per-user "my match" page

New route at `/play/` (or `/my-match/`). Subscribes to `host:all`, finds the container whose roster contains any of the logged-in user's gamertags (from M7c), then renders the existing kiosk iframe + controller UI for that container. Renders blank/idle state otherwise. Auto-refreshes if the user's gamertag appears on a different container later.

## 9c. WS auth narrowing

Today `host:<name>` requires admin. Extend the room guard so a non-admin user is permitted to join `host:<name>` if they have a gamertag in that container's roster. New room-level guard in [internal/websocket/rooms/host.go](../internal/websocket/rooms/host.go) or a new sibling guard, consuming the new role helpers from M8c so admins always get in regardless of roster membership. Keep `host:all` admin-only (it's a cross-instance summary).

Smoke test: 4-container match, 4 logged-in users with one gamertag each, each gamertag mapped to one container's local Xbox machine. Each user opens `/play/` → sees only their container's kiosk. Admin opens `/play/` while not playing → blank state, but admin can still hit `/containers/<name>/` directly. User logs in but isn't in any active match → blank state.
