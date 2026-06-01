# M22 — Moderation + audit log

> **Status:** Done
> **Started:** 2026-05-26
> **Completed:** 2026-05-31
> **Depends on:** M07 + [ADR-0002 (unified `audit_log` collection)](../decisions/0002-unified-audit-log-collection.md), which fixes the moderation/history shape M22 builds on.

## Goal

Stand up the moderation infrastructure that turns M07's identity schema into something safe to expose. Gamertags and team names become 4-state (approved / allowed / pending / blocked) with an admin queue, edits flip back to "allowed" for re-check, and a reserved-name pre-list catches obvious bad submissions before they hit the public view. Audit shape comes from ADR-0002 (unified `audit_log` collection): per-row state columns + an event-sourced trail in `audit_log`.

Implicitly raises trust in the platform: users can submit content freely (default = visible), but admins have a single review queue and an audit trail that survives both admin succession and per-row deletion.

## Scope

**In:**

- Gamertag 4-state moderation (`approved` / `allowed` / `pending` / `blocked`) with state-transition rules from 7h.
- Team-name 4-state moderation (mirror of gamertag).
- Team name history (rename tracking — every name a team has carried, with timestamps).
- Reserved-name pre-list (admin-curated patterns; matches set `status = pending` rather than auto-blocking, to avoid false positives like "basement" → contains "ass").
- Audit columns or audit-log collection (per 7h).
- Admin queue UI (`/admin/players/` Gamertags view + `/admin/rosters/` Teams view): "Pending" + "Needs double-check" sub-lists.
- Migration: existing `blocked: bool` gamertag rows backfilled as `approved` with `reviewed_by = null`.

**Out:**

- User-side reporting/flagging UI (deferred — community is small enough that it's not urgent).
- Auto-block automation on the pre-list (we flag for review only).
- Per-user "needs review on every tag" trust level (lands in M8 alongside the role restructure).

## Actions

- [ ] Materialize the `audit_log` collection per [ADR-0002](../decisions/0002-unified-audit-log-collection.md). Build the `internal/audit/` write helper + per-action payload structs.
- [ ] Tasks #7, #10, #12, #17 from the open task list — see those for sub-bullets.
- [ ] Migrate existing `blocked: bool` rows → 4-state `status`; synthesize `audit_log` rows (`actor=null, action='block'`) for the historical block events.
- [ ] Wire the admin queue views into the existing /admin/players/ + /admin/rosters/ pages (split landed in M7f).

## Verification

- Create a gamertag with the pre-list trigger pattern → status lands as `pending`, not visible to other users.
- Admin approves from the queue → status flips to `approved`, audit entry recorded.
- Admin blocks a previously-approved tag → status flips to `blocked`, owner can no longer edit or delete it, owner can't re-add the same `(user, sanitized)`.
- Owner edits an `approved` tag → status flips back to `allowed` for re-check.
- Team renames produce a row in the team-name-history surface; the team page shows "formerly known as X" entries.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-05-26: created — drafted alongside M07 scope expansion. Hard-depends on the 7h writeup for the audit-column shape.
- 2026-05-26: ADR-0002 lands the audit-log shape decision (unified `audit_log` collection). M22 unblocked.
- 2026-05-26: 22a — audit-log foundation. New `audit_log` collection registered in `internal/pocketbase/schema/audit_log.go` per ADR-0002 (actor / target*collection / target_id / action / payload_json / created + the three documented indexes; listRule admin-only, mutate rules nil). New `internal/audit/` package exposes `Write(app, actor, action, target, payload)` (in-hook ergonomic) + `WriteRef(app, actor, action, collection, id, payload)` (for post-delete or synthetic rows); callers always pass `actor` explicitly. `action.go` declares the `Action` type and the one-file-per-action convention (`action*<verb>.go`pairs an`ActionXxx`constant with an`XxxPayload` struct) — substage 22a ships only the type + convention, the first real action files land with 22b. Unit tests cover both helpers + nil-actor + validation. No moderation features in this slice; gamertag/team 4-state, rename history, and reserved-name pre-list still pending.
- 2026-05-30: 22c — admin review queue UI. `/admin/players/` swaps the single flat list for three Skeleton Tabs sub-lists with per-tab counts: Pending (status=pending; empty pre-M22e), Needs double-check (status=allowed — every new submission lands here pre-approval, plus auto-downgrades), and All. Single-row Approve action (preset-tonal-success check icon) sits next to the existing Block toggle and writes `status="approved"` — the backend hook emits the `ActionApprove` audit row with the requesting admin as actor. Frontend-only slice: no schema or hook changes, no bulk multi-select (deferred), no per-row audit history drawer (deferred to 22d/e). Verified end-to-end via Playwright: Pending tab shows "No pending review", Needs double-check shows the seeded admin gamertag with Approve / Block / Edit / Delete buttons, All tab matches the prior single-list view.
- 2026-05-31: M22 closed. All documented verification criteria met across 22a–22e (audit-log foundation, gamertag 4-state, admin queue UI, team-name 4-state + rename history capture, reserved-name pre-list). Team-page "formerly known as" reader that consumes the ActionRename rows is deferred to ride alongside the public team page work (currently a placeholder pointing at M23). Community-report flow stays out-of-scope per the original M22 doc — defer until the community grows past self-service moderation.
- 2026-05-31: 22e — reserved-name pre-list. New `reserved_names` collection (pattern + description + created_by) curated by admins; new `internal/reservednames` package's `Match(app, candidate)` does substring-on-lowercase matching ("ass" matches "Sassy" and "BASS"). Two new hooks (`gamertags_reserved_name.go`, `teams_reserved_name.go`) run on create + update of the moderated field, set `status="pending"` on a hit, and rely on alphabetical init() ordering to fire before the existing `*_status_transitions` hooks so the default-status logic sees the pending value already set. Hook is best-effort — a Match error logs but doesn't fail the underlying record write. New `/admin/reserved-names/` admin page (DataTable + add/delete dialog) with a nav link under the Admin group. Per the M22 doc, matches flag for review rather than auto-blocking — false positives are an accepted cost. The Pending tab on `/admin/players/` and `/admin/rosters/` Teams now has a real data source for the first time. End-to-end verified live: add pattern "evil" → create gamertag "eviltag" lands status="pending", create gamertag "cleantag" lands status="allowed", create team "EvilTeam" lands status="pending". M22 verification criteria (gamertag/team 4-state, admin queue UI, reserved-name pre-list, rename history capture) all met; team-name 4-state moderation column on the Teams tab now closes out.
- 2026-05-31: 22d — team-name 4-state moderation + rename history capture. Mirror of 22b+22c for the teams collection: `schema/teams.go` adds the `status` SelectField (idempotent migration backfills any existing rows to `allowed`; no synthetic backfill since teams never had a `blocked` column), and PB rules swap owner update/delete to `(created_by = @request.auth.id && status != "blocked") || @request.auth.isAdmin = true`. New `hooks/teams_status_transitions.go` mirrors the gamertags hook with one addition: every `name` change emits an `ActionRename` audit row (always, regardless of status), so the eventual team-page "formerly known as" reader can walk `audit_log` filtered to `(target_collection="teams", action="rename")` without an extra column. The auto-downgrade case (owner renames an approved team) produces two audit rows — `ActionRename` for the data change + `ActionEdit` for the moderation transition — so the rename-history reader and the moderation queue don't have to disambiguate each other's events. New `internal/audit/action_rename.go` declares ActionRename + RenamePayload (PrevName, NewName, ByAdmin). `/api/me` adds `status` to the teams payload; `/settings/` renders the team status badge inline with the team name; `/admin/rosters/` Teams tab gets the same three-tab queue + Approve row action as M22c. End-to-end verified live: create team → status="allowed"; approve → ActionApprove row with prev_status="allowed"; rename approved team → status downgrades to "allowed", audit log shows both ActionRename (by_admin=true) AND ActionEdit (field:"name", prev/new name + statuses).
- 2026-05-29: 22b — gamertag 4-state moderation. Migration in `schema/gamertags.go` swaps the M7 `blocked: bool` for a `status` SelectField (`approved` / `allowed` / `pending` / `blocked`), backfills existing rows + emits synthetic `action="block"` audit rows for prior blocks (actor=nil), and drops the `blocked` column once every row carries a non-empty status. New `hooks/gamertags_status_transitions.go` defaults new rows to `allowed`, audits block/unblock/approve transitions via `audit.Write` with the requesting user as actor, auto-downgrades `approved` rows back to `allowed` when the owner edits `tag` (recorded as `ActionEdit` with prev/new tag + status), and rejects non-admin direct status writes with a 400. First real action files land: `action_block.go`, `action_unblock.go`, `action_approve.go`, `action_edit.go`. PB rules swap `blocked = false` → `status != "blocked"`. `/api/me` returns `status: string`; `/admin/players/` block toggle now flips `blocked ↔ allowed`, edit dialog is tag-only (status transitions go through the toggle or M22c admin queue), settings page renders the 4-state badge. End-to-end verified: admin block → audit row `{action:"block", actor:admin, payload:{prev_status:"allowed"}}`; admin approve then owner tag-edit → audit row `{action:"edit", payload:{field:"tag", prev/new values + statuses}}`. Admin queue UI (pending review surface, bulk approve/block) deferred to 22c.
- 2026-05-31: Released as v0.7.0 per ADR-0001 (CHANGELOG [Unreleased] → [0.7.0] - 2026-05-31, git tag v0.7.0). Bundled with M07 in the same release cut.
