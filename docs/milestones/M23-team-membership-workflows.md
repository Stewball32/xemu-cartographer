# M23 — Team membership workflows

> **Status:** Planned
> **Started:** —
> **Completed:** —
> **Depends on:** M07 (rosters + owner/manager roles), M22 (so blocked tags + blocked teams don't pollute invite flows)

## Goal

Make team membership a two-way negotiation backed by a real notification surface. Owners/managers can invite users; users can request to join teams; rostered users can self-leave; owners/managers can remove others. None of this is reachable today because there's no delivery mechanism — adding `notifications` as a first-class surface unblocks invitations + sets the foundation for future audit-flavored alerts (block notices, ban notices).

Once shipped, the platform feels alive in a way M7's static schema alone can't manage: a user receives a "you were invited to NorCal Halo" bell badge, clicks it, picks the gamertag they want to represent under, and the roster row materializes.

## Scope

**In:**

- `notifications` collection: user (recipient), type, payload_json, read (bool), read_at, created.
- Header bell icon with unread count + dropdown showing recent N, "mark all read", link to a full `/notifications/` page.
- `team_membership_requests` collection: team, user, gamertag, direction (`invited` | `requested`), status (`pending` | `accepted` | `declined` | `expired` | `cancelled`), initiated_by, responded_by, expires_at, created.
- Invitation flow: owner/manager triggers from `/u/[username]/` "Invite to team" CTA → creates `direction=invited` request + notification for the user.
- Join-request flow: user triggers from `/teams/[slug]/` "Request to join" CTA → creates `direction=requested` request + notification for the team's owners/managers.
- Accept: creates a roster row with the chosen gamertag.
- Decline / cancel: marks the request, notification to the other party.
- Self-leave: any user can set `left_at` on a roster row they own (own = gamertag.user matches them).
- Owner/manager removal: can set `left_at` on others' roster rows for their team.
- PB rules: list/view own notifications only; mark-read on own only; admin override.

**Out:**

- WS push for notifications (poll-on-focus is fine for MVP; WS upgrade is a follow-up).
- Email delivery of notifications (in-app only).
- Rate limits on invite spam (flagged for whenever registration opens broadly).
- Reasons / cooldowns on decline (the request just dies; requester can retry).

## Actions

- [ ] Tasks #11 + #20 from the open task list — see those for sub-bullets.
- [ ] Notification CRUD + bell UI lands first; invitations + requests build on top.
- [ ] Soft-deleted users' notifications get garbage-collected at delete time.

## Verification

- Owner invites a user → notification appears in the user's bell → user accepts with their chosen gamertag → roster row materializes, both parties see the team in their `/api/me`.
- User requests to join a team → owner gets a notification → owner approves → roster row materializes.
- User self-leaves → `left_at` stamped, team page shows them in the "former members" section.
- Owner removes another member → `left_at` stamped, notification sent to the removed user.
- Decline / cancel paths produce notifications without creating roster rows.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-05-26: created — drafted alongside M07 scope expansion + M22 draft. Notification system is a hard prerequisite for the invite/request flows, so both land in this milestone rather than spanning two.
- 2026-06-01: M23 opened on `wip/milestone-23`. Locked decisions before substages started: (a) one branch + one PR cut at milestone close (no intermediate merges to main); (b) `audit_log` stays moderation-only — add a new `team_log` collection in M23c for membership/rename history, with the team page reading from it instead of `audit_log` (the M22d `audit.Write(ActionRename)` call stays, twin-write is fine); (c) `default_gamertag` is the social handle, so the accept routes resolve it server-side and the `team_membership_requests` collection has no `gamertag` column. Substages: 23a notifications foundation, 23b bell UI + /notifications/ page, 23c team_log + team_membership_requests + invite/request routes + public team/profile pages, 23d accept/decline/cancel + self-leave + owner-remove, 23e soft-delete GC + close-out docs.
- 2026-06-01: 23a — notifications foundation. New `notifications` collection (`user` relation, `type` text, `payload_json` JSON, `read` bool, `read_at` date, `created` autodate) with the two documented indexes (`user, read, created DESC` for bell counts, `user, created DESC` for the full-page walk). Rules: list/view/update = recipient or admin; create/delete = nil (writes flow through the server-side helper which bypasses rules via `app.Save`). New `internal/notifications` package exposes `Notify(app, user, type, payload)` mirroring `audit.Write`. Nine notification-type files cover the M23c–d action surface: `team_invite`, `team_invite_accepted/declined/cancelled`, `team_join_request`, `team_join_request_accepted/declined/cancelled`, `team_removed`. New `hooks/notifications_field_lock.go` (OnRecordUpdateRequest) restricts non-admin updates to the `read` field and auto-stamps `read_at` on the false→true transition; admin writes skip the gate. `/api/me` returns `notifications_unread_count` so the bell badge has its source on every hit. Unit tests cover the helper minimal path, nil-payload path, and validation. No UI in this slice — the bell + page land in 23b.
- 2026-06-01: 23b — bell UI + /notifications/ page. New `notifications` runes singleton (`sveltekit/src/lib/stores/notifications.svelte.ts`): tracks `unreadCount` + `recent[]`, refreshes on initial mount + window `focus` + post-action triggers (no `setInterval` — the M23 doc's "poll-on-focus is fine for MVP" trade), subscribes to `pb.authStore.onChange` so the badge clears on logout and refreshes on login. New `HeaderBell` component (Skeleton Popover anchored to a `BellIcon` + error-500 unread badge, capped at "99+"): top-20 dropdown rendered via the shared `notification-render.ts` helper which parses opaque `payload_json` into typed titles/descriptions/hrefs per type — keeps the bell free of per-type logic and pre-builds the click target. Mark-as-read on click; mark-all-read button. Full-page `/routes/notifications/` mirrors the dropdown's row render but groups by Today / This week / Older for skimmability, with a 50-row first page and an explicit Refresh + Mark all read in the page header. Header.svelte gets the bell wedged between ModeToggle and the user avatar, gated on `auth.isLoggedIn`. No invite/request actions yet — those wire up in 23d alongside the route handlers.
