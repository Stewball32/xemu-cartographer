# M22 — Moderation + audit log

> **Status:** Planned
> **Started:** —
> **Completed:** —
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
