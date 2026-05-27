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
