# ADR-0002 — Unified `audit_log` collection (vs per-row audit columns)

> **Status:** Accepted
> **Date:** 2026-05-26

## Context

M7 (identity schemas) closes with multiple downstream milestones planning to record actor-tracked moderation, history, and grant events:

- **M22 (Moderation + audit log)** — gamertag 4-state moderation (`approved` / `allowed` / `pending` / `blocked`), team-name 4-state moderation, team-name rename history, and the reserved-name pre-list. Each transition needs `actor`, `target`, `old`/`new`, `reason?`, `created`.
- **M8 (Roles + permissions)** — role grants, user bans, timeouts. Each needs `actor`, `target_user`, `role`/`action`, `expires_at?`.
- **M13+ (Persistence foundation)** — game persistence may want admin overrides (manual game corrections, voided games) recorded for replay-of-record disputes.

Two natural shapes:

1. **Per-row audit columns.** Add `created_by` / `updated_by` / `reviewed_by` / `state_changed_at` / `last_state_actor` on each collection that needs them. Schema is typed; consumers don't need to know how to decode a generic payload.
2. **Unified `audit_log` collection.** One event-sourced table — `actor`, `target_collection`, `target_id`, `action`, `payload_json`, `created`. Every state change in every collection writes a row here.

M7 itself stayed at one shape (a `blocked: bool` on gamertags with no actor column) because the cross-collection demand only crystallized as M22 + M8 + M13 were drafted. The decision now decides whether each follow-on milestone independently invents an audit column set, or whether they all write through a single surface.

The deciding question: **how often will we need to read across collections?** The moderation dashboard's "review queue across gamertags + team names" view, an "admin activity in the last 24h" report, and incident post-mortems all want a cross-collection feed. Per-row columns force `UNION ALL` reconstruction at every read site.

## Decision

Adopt a single `audit_log` collection. Shape:

```
audit_log
  id                text (pk)
  actor             relation → users (nullable; null = system/automated)
  target_collection text          (e.g. "gamertags", "teams", "users", "rosters")
  target_id         text          (PB record id; foreign-key-shaped but unconstrained — preserves rows after target delete)
  action            text          (enum-flavored: "create" / "update" / "delete" / "approve" / "block" / "unblock" / "rename" / "role_grant" / "role_revoke" / "ban" / "timeout")
  payload_json      json          (action-specific data: old/new values, reason text, expiry timestamps, etc.)
  created           autodate
```

Indexes: `(target_collection, target_id, created DESC)` for per-row history walks; `(actor, created DESC)` for "what did this admin do recently"; `(action, created DESC)` for queue-flavored reads.

Write path: PB record hooks on the affected collections call a shared `audit.Write(app, actor, action, target, payload)` helper. The helper hides the JSON shape construction so per-action call sites stay terse.

Read path: M22's moderation queue reads the latest row per `(target_collection, target_id)` filtered by `action IN ('pending', 'approve', 'block')`. The team-page "formerly known as" reads `WHERE target_collection='teams' AND target_id=$id AND action='rename' ORDER BY created`. The admin dashboard reads `WHERE actor=$me AND created > now-24h`. All three are single-collection queries.

## Consequences

**Positive.**

- One read pattern for the moderation surface in M22 — the four sub-features (gamertag state, team-name state, name history, reserved-name flagging) all walk the same table.
- Cross-collection queries trivial. "All admin actions in the last 24h" is one query, not a `UNION ALL` across N tables.
- Adding a new audit-emitting collection (M13 game corrections, future M19 offset overrides) requires zero schema migration — just call `audit.Write(...)`.
- The audit trail survives target deletion because `target_id` is unconstrained text. A hard-deleted user's history of admin actions remains queryable.
- Payload shape is per-action, so we can add a new field (e.g. "duration" on a timeout) without migrating any existing rows.

**Negative.**

- `payload_json` is opaque to the DB. Consumers must know the per-action schema; a typo at write time silently corrupts that row's payload. Mitigation: centralize writes through the `audit.Write` helper and define typed action enums + payload structs in `internal/audit/` (lands in M22).
- Cross-collection queries that need to JOIN target rows are awkward — `target_id` is a string, not a `relation`, so PB's `expand` mechanic doesn't apply. M22's read paths will need explicit lookups per target type.
- Volume scales with mutations across the whole system. At current scale (single-instance hobbyist platform) this is invisible; if the platform grows to 1000s of mutations/sec it would need partitioning or rollup. Out of scope for M7-M30.

**Neutral.**

- Per-collection columns can still be added selectively when a column makes the schema noticeably easier to query at a hot path (e.g., `users.last_login_at` doesn't need to live in `audit_log`). The unified collection is the default for _moderation/admin-action_ writes, not a blanket rule that bans per-collection metadata.
- The 4-state moderation fields (`status`, `reviewed_by`, `reviewed_at`) still live on gamertags + teams in M22 — they're the _current_ state. The transition history is what lives in `audit_log`. Treat the per-row state field as a materialized projection of the latest applicable `audit_log` row.

## Notes

- M22 implementation owns the actual collection migration + the `internal/audit/` write helper; this ADR commits to the shape so M22 doesn't relitigate it.
- M7's existing `gamertags.blocked: bool` becomes a 4-state `status` enum in M22's migration; the historical `blocked=true` rows backfill as `status='blocked'` with `audit_log` rows synthesized with `actor=null, action='block'`.
- This ADR supersedes the open question in M7's 7h sub-stage ("short ADR or inline section"). M22's "Depends on" is now satisfied.
