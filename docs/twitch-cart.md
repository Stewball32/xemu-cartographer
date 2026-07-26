# Twitch functionality for cart (roadmap)

Status: **planned, not built.** Cart today has only the Twitch OAuth *login*
provider (`internal/pocketbase/oauth/twitch.go`). The features below are future
work. Logged 2026-07-26.

> Distinct from the **site's** Twitch integration (go-live EventSub + "Live Now"
> board), which is already implemented in `norcal-halo-site`. Cart's angle is
> different: it leverages cart's live game-state awareness.

## Feature

Two capabilities, both driven by cart's real-time scraper game state:

1. **Twitch chat bot** — announce live game state in chat (map, score, notable
   events) and respond to viewer commands.
2. **Auto-clip / highlight creation** — trigger a clip at the exact moment of a
   notable play detected from game state (comeback win, killing spree, final
   kill).

## Open implementation question — clip capture (Stewart undecided)

How to actually capture the clip:

| Approach | What you get | Cost / requirement |
| --- | --- | --- |
| **Twitch Helix Clips API** | ~last 30s, clip hosted on Twitch (shareable URL) | Needs `clips:edit` scope; quality/length fixed by Twitch |
| **OBS (obs-websocket / replay-buffer save)** | Local, higher-quality file, arbitrary length | No Twitch clip scope; **requires OBS running with the replay buffer enabled** |

The OBS route fits naturally since **cart already drives the OBS overlays**, so
obs-websocket is already in the picture. Decision pending.

## Credentials / scopes matrix

One **shared Twitch application** (single client id + secret) is reusable across
site + cart. On top of that, each capability needs its own auth:

| Capability | Auth needed | Scopes / config |
| --- | --- | --- |
| **Shared app** | Twitch application | `TWITCH_CLIENT_ID` + `TWITCH_CLIENT_SECRET` (one pair, site + cart) |
| **Chat bot** | Dedicated Twitch **bot account** + user OAuth token (with refresh token stored) | `chat:read` + `chat:edit` — or the EventSub-chat equivalents `user:read:chat` + `user:write:chat` + `channel:bot` |
| **Clips via Twitch API** | **Broadcaster** user OAuth | `clips:edit` |
| **Clips via OBS** | No Twitch scope | obs-websocket host/port + password; OBS replay buffer enabled |
| **Optional go-live / "Live Now" on cart** | Same app-level EventSub creds as the site | client id/secret + self-chosen EventSub secret + public callback URL (see the site's `TWITCH_EVENTSUB_SECRET` / `TWITCH_EVENTSUB_CALLBACK`) |

Notes:
- The chat bot's token must be for a **separate bot account**, not the
  broadcaster — and its refresh token persisted, since user tokens expire.
- Clips-via-API needs a **broadcaster** token (`clips:edit`); clips-via-OBS needs
  **no Twitch scope at all** — the tradeoff above is partly a credentials
  tradeoff too.
- If cart ever adds go-live/Live Now, it reuses the *same* Twitch app as the
  site; only the per-tier EventSub secret + callback URL differ.
