# Milestone 6 — Frontend polish (theme + auth-refresh fix + debug revamp)

> The site has accumulated style drift, the debug page needs both more data and better organization, and a long-standing auth-hydration race redirects admin pages to home on hard refresh. Cluster these three so the visual + dev-tooling foundation is solid before bigger feature work (M9 kiosk, M10 overlays).

## 6a. Auth-refresh redirect fix (frontend + backend audit)

Hard-refreshing `/containers/` or `/admin/debug/` bounces to home.

*Frontend root cause (primary).* In [sveltekit/src/lib/stores/auth.svelte.ts](../sveltekit/src/lib/stores/auth.svelte.ts), `auth.ready` is captured at module init, but on a cold CSR hydration the `+page.ts` load function calls `requireAdmin()` ([sveltekit/src/lib/utils/guards.ts](../sveltekit/src/lib/utils/guards.ts)) before the auth store has hydrated `isAdmin` from `/api/me`. Both [routes/admin/debug/+page.ts](../sveltekit/src/routes/admin/debug/+page.ts) and [routes/admin/debug/[name]/+page.ts](../sveltekit/src/routes/admin/debug/[name]/+page.ts) `await auth.ready` already, but the promise resolved instantly with stale state. Options: (a) move the `/api/me` fetch into a root `+layout.ts` load that all child routes inherit, (b) make `auth.ready` rebuild on every `pb.authStore` change rather than capture once. Decide during implementation.

*Backend gating audit.* Sweep [internal/pocketbase/routes/](../internal/pocketbase/routes/) for parallel inconsistencies — guards that should fire but don't, or fire too eagerly. Focus on:
- [internal/pocketbase/routes/middleware/auth.go](../internal/pocketbase/routes/middleware/auth.go) and [admin.go](../internal/pocketbase/routes/middleware/admin.go) — confirm the `RequireAuth` + `RequireAdmin` chain matches what's documented in CLAUDE.md (PB superusers + `users.isAdmin=true`).
- [internal/pocketbase/routes/admin/](../internal/pocketbase/routes/admin/) group registration — every admin route should inherit the chain, not re-add or skip.
- [internal/pocketbase/routes/me.go](../internal/pocketbase/routes/me.go) — currently exposes `{isAdmin, isSuperuser}` to the caller; verify it doesn't leak other users' admin status.
- [internal/pocketbase/routes/containers/](../internal/pocketbase/routes/containers/), [scraper/](../internal/pocketbase/routes/scraper/), [xemu/](../internal/pocketbase/routes/xemu/) — confirm each registered handler is gated.
- [internal/pocketbase/routes/allroutes.go](../internal/pocketbase/routes/allroutes.go) and [allgroups.go](../internal/pocketbase/routes/allgroups.go) — registration order.

If the frontend fix exposes a backend route that should have been guarded but wasn't, fix both at once.

Smoke test: hard-refresh `/containers/` and `/admin/debug/<name>/` while logged in as admin → page loads, no home bounce. Hit each `/api/admin/*` endpoint as anon, as a non-admin user, and as an admin → 401/403/200 respectively.

## 6b. Custom Skeleton theme + style consistency

Today the cerberus theme is loaded statically via [sveltekit/src/routes/layout.css](../sveltekit/src/routes/layout.css) (`@import '@skeletonlabs/skeleton/themes/cerberus'`) and the root sets `data-theme="cerberus"` in [sveltekit/src/app.html](../sveltekit/src/app.html). Define a project-branded theme — likely via Skeleton v4's theme generator or a hand-written `tailwind.config.ts` with custom design tokens. Audit pages for inconsistent spacing, button variants, and card patterns; centralize repeating chrome into reusable components. Pages to audit: `/`, `/login/`, `/containers/`, `/containers/[name]/`, `/admin/debug/`, `/admin/debug/[name]/`, `/overlays/players/`.

Smoke test: visual diff before/after; dark + light mode both render cleanly; OBS overlay backdrop stays transparent (overlays must remain unaffected by theme background).

## 6c. Debug page revamp + scraped-data validation

Existing tabs (Overview / Game / Tick / Events / Probe / Raw JSON) in [sveltekit/src/routes/admin/debug/[name]/+page.svelte](../sveltekit/src/routes/admin/debug/[name]/+page.svelte) and components in [sveltekit/src/lib/components/debug/](../sveltekit/src/lib/components/debug/). Scope:

- **Data coverage audit.** Walk every field surfaced by the Halo: CE reader ([internal/scraper/haloce/reader.go](../internal/scraper/haloce/reader.go), `offsets.go`, `offsets_reference.go`); confirm each renders somewhere in the debug UI. Promote currently-buried fields where useful.
- **Restyle.** Apply M6b theme; replace `KvCard` / raw JSON dumps with structured tables for high-volume fields (objects, projectiles).
- **Verification harness.** For fields tagged `unverified` in the offset table, surface a per-field "looks plausible / clearly broken / unknown" annotation as a manual-validation pass, feeding M19's runtime offset validation.
- **Probe tab.** Audit the existing probe outputs (`BuildScoreProbe`, `LastStateInputs`) and add probes for any field cluster currently untrusted.

Smoke test: 4-instance system-link match (same harness as M5 5c+5d+5e smoke), walk every tab on every instance, log any field that displays empty/zero/garbage and create offset-investigation follow-ups for M19.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-05-17: M6c — Overview tab gains a Handshake section showing the v2 hello envelope (protocol/classes/instances). Placeholder tabs added for `container`, `debug`, `objects`, `scenario`, and `summary` so the debug page's tab strip matches the planned envelope-class taxonomy.
