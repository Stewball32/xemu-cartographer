# Gamertag identity system

> **Status:** Built + live-verified (2026-06-25, branch `feat/gamertag-system`).
> H2 profile + gametype generation work end-to-end; CE profile generation is a
> deferred scaffold pending the CE player-name/profile research. The 20-byte
> save digest is template-patched (preserved), inheriting the LAN-hub
> generator's one open item.

A unified **gamertag = your in-game name** identity layer on top of the LAN-hub
[`halosave`](../lan-hub/README.md) generator. The gamertag is a field on the
**user** record (separate from the account `username`) and drives **two Halo
player profiles** (CE + H2), edited side by side but stored separately. Whenever
the gamertag, a profile, or a gametype changes, the hub **regenerates the
actual, signed Xbox save file** and stores it on the record, ready for the nxdk
LAN client to pull to the box.

## Gamertag = a field on the user (not the username)

The gamertag is **separate from the account `username`** (which is a normal
login identifier with no Halo constraints). It lives on the `users` record
(`users.gamertag`) as the single source of truth, and is the in-game name
written into BOTH player profiles — the profile records do **not** store it; the
generate hooks resolve it via the `user` relation, and `users_gamertag_regen`
re-generates the profiles whenever it changes.

It is length-capped to **`schema.GamertagMaxLen` (113 chars)** = the shorter of
the two games' name fields (a gamertag must fit both). H2's `profile` name field
is a 0xE4-byte (228) UTF-16LE NUL-terminated buffer (`halosave/h2profile.go
h2pNameBuf`, cross-checked against 4 real profiles) → 228/2 − 1 NUL = 113 chars.
CE's is ~11 chars (FORMATS.md §2) but the exact CE name source is still being
researched, so **until CE lands the cap defaults to the H2 limit** and is the one
knob to tighten to CE's smaller value. The frontend mirrors the number
(`GAMERTAG_MAX_LEN`).

```
 Browser (user)                 Browser (organizer)            nxdk client (Xbox, LAN)
   /gamertag/                     /organizer/gametypes/          GET /api/lan/saves/identity/<gt>
   edit CE+H2 profile             /organizer/games/              → manifest of profiles+gametypes+games
        │ pb.collection(...).create/update      │ create / upload        │ GET /api/lan/saves/file/<kind>/<id>
        ▼                                        ▼                        ▼
 ┌──────────────────────────────────────────────────────────────────────────────┐
 │  PocketBase collections: ce_profiles · h2_profiles · gametypes · game_titles   │
 │  generate-on-save hooks  →  internal/saveartifact  →  internal/halosave        │
 │  store: save_bundle (tar of the FATX save dir) + save_info (sha1/sizes/digest) │
 └──────────────────────────────────────────────────────────────────────────────┘
```

## What you get

| Surface | Route / endpoint | Who |
| --- | --- | --- |
| Gamertag editor (CE + H2 profiles, side by side) | `/gamertag/` | any signed-in user (own profiles) |
| Gametype library (CE + H2 variants) | `/organizer/gametypes/` | organizer or admin |
| Games (XBE) uploads | `/organizer/games/` | organizer or admin |
| Per-gamertag download manifest | `GET /api/lan/saves/identity/{gamertag}` | LAN-token / admin |
| List gamertags with a profile | `GET /api/lan/saves/identity` | LAN-token / admin |
| Stream a stored save/upload | `GET /api/lan/saves/file/{kind}/{id}` | LAN-token / admin |

`{kind}` ∈ `h2-profile` · `ce-profile` · `gametype` · `game`.

## Data model (PocketBase)

Four new collections, registered from `schema/identity.go` **phase 5** (after
`roles` + `user_roles`, because their rules embed the role subqueries).

| Collection | Key fields | Generated? | Access (PB rules) |
| --- | --- | --- | --- |
| `users` (built-in) | + `gamertag` (text, ≤113) — the in-game name, separate from `username` | — | self read/write; admin |
| `ce_profiles` | `user` (unique), `settings` (json), `save_bundle` (file), `save_info` (json) — gamertag read from the user | **deferred** (no CE profile file format yet) | owner read/write; admin anything |
| `h2_profiles` | `user` (unique), `appearance` (json byte-map), `save_bundle`, `save_info` — gamertag read from the user | yes — 500-byte `profile` + `SaveMeta.xbx`, tar'd | owner read/write; admin anything |
| `gametypes` | `title` (ce/h2), `engine`, `name`, `settings` (json), `save_bundle`, `save_info`, `created_by` | yes — CE `blam.lst` / H2 mode payload + `SaveMeta.xbx` | read: any authed; write: organizer/admin |
| `game_titles` | `name`, `description`, `file` (≤2 GiB), `created_by` | n/a — plain upload | read: any authed; write: organizer/admin |

> **Naming:** the XBE library is `game_titles`, **not** `games` — the M13 `games`
> collection (contests played) already owns that name. They are unrelated.

The `organizer` role is added to the M08 baseline roles (level 30). Because
`/api/me` already returns `roles` generically, `auth.hasRole('organizer')` works
on the frontend with no extra wiring; the PB-rule constant `organizerOrAdmin`
lives in `schema/rules.go`.

## Generate-on-save

Generation is a PocketBase **record hook**, not a route — so it fires on every
save regardless of who writes the record (SDK, route, seeder):

- `OnRecordCreate`/`OnRecordUpdate` on `h2_profiles` / `gametypes` →
  `saveartifact.Build(req)` → attach the tar to `save_bundle` and the metadata
  to `save_info`, **in the same transaction** (`hooks/save_artifact.go`
  `attachBundle`). An invalid input (e.g. an out-of-range appearance byte) makes
  the generator error, which rejects the save — never store an unbuildable
  record.
- `ce_profiles` runs the same hook shape but **stamps `save_info.deferred = true`
  and writes no file** — CE has no standalone MP profile save format (see below).
- The profile hooks resolve the in-game name from **`users.gamertag` via the
  `user` relation** (`userGamertag`), not a per-profile field — so a user with no
  gamertag set yet can't create a profile (the save is rejected). When the
  gamertag changes, `hooks/users_gamertag_regen.go` re-saves the user's profiles
  so their files rebuild against the new name.

`internal/saveartifact` is the pure seam: it maps the typed settings → a
`halosave.BuildRequest`, calls `halosave.Build`, and tars the result at
`UDATA/<titleID>/<dir>/<file>` (unpack relative to the Xbox `E:\` root). It is
unit-tested directly; the hook→file-field→read-back path has a PocketBase
integration test (`hooks/save_artifact_test.go`, which also covers the
gamertag-rename cascade).

## The CE profile is a scaffold (deferred — pending research)

Halo: CE has **no standalone multiplayer player-profile save file** — confirmed
in the LAN-hub format reverse-engineering ([FORMATS.md §3](../lan-hub/reference/FORMATS.md)).
The editable CE surface is the *gametype*. So:

- `ce_profiles` stores the gamertag + a pluggable `settings` blob now.
- The generate hook is deliberately a no-op-with-marker (`save_info.deferred`).
- **The single seam:** when the separate *Halo CE Xbox player-name/profile
  source* research lands, fill `saveartifact` with a CE-profile builder and swap
  the `ce_profiles` hook's deferred stamp for the same `attachBundle(...)` call
  the H2 hook uses. The record, editor, download manifest, and serve endpoint are
  already wired — it is a one-function change.

The CE editor (`/gamertag/`, Halo: CE column) renders the gamertag now and a
clearly-labelled "pending research" panel where the appearance/controls fields
will drop in.

## H2 customization field set

The H2 profile editor exposes the reverse-engineered appearance/controller bytes
(`halosave.H2ProfileFields`), grouped Armor / Emblem / Controls. Labels are
**provisional** (2-sample reverse engineering — surfaced as such in the UI).
More samples would finish labelling; the byte offsets are stable. H2 gametypes
map name + score limit (1 sample); other settings are preserved from the
template.

## The 20-byte digest

Inherited verbatim from the LAN hub: every payload save carries a 20-byte
content-dependent digest whose algorithm isn't cracked offline, so generation is
**template-patch** (the digest is preserved from a real sample). Unchanged
clones are byte-identical to real saves; edited-settings files carry a stale
digest. Whether Halo re-verifies it on load is the one unverified question (a
~10-min live-xemu probe — see [the LAN-hub de-risk verdict](../lan-hub/reference/DE-RISK-VERDICT.md)).
`halosave.RecomputeDigest` remains the single hook for when it's resolved.

## nxdk client wiring (follow-up)

The per-gamertag endpoints are the contract the device client consumes:

1. `GET /api/lan/saves/identity/{gamertag}` → JSON manifest: `ce_profile`,
   `h2_profile`, `gametypes[]`, `games[]`, each with a `download_url` and (for
   saves) `info` (fatx dir, file sizes, sha1s, digest status).
2. For each item, `GET /api/lan/saves/file/{kind}/{id}` → streams the save
   bundle (a tar to unpack at `E:\`) or the raw game upload. Streaming uses
   `http.ServeContent`, so Range requests work for large game files.

What the client still needs (deferred): drive these two endpoints, unpack each
profile/gametype tar onto the FATX `E:` partition (the existing `/download`
disk-space pre-flight via `free_bytes` applies), and place game uploads where
the launcher expects them. Auth: the LAN endpoints accept an admin JWT or the
optional `LAN_SAVES_TOKEN` (open on a trusted LAN by default).

## Code map

| Path | Role |
| --- | --- |
| `internal/saveartifact` | Pure: typed settings → `halosave.BuildRequest` → SaveSet + tar bundle + lean `Info`. Unit-tested. |
| `internal/pocketbase/schema/{ce_profiles,h2_profiles,gametypes,game_titles}.go` | The four collections + access rules. |
| `internal/pocketbase/schema/{roles,rules,identity}.go` | `organizer` role, `organizerOrAdmin` rule, phase-5 registration. |
| `internal/pocketbase/hooks/{save_artifact,h2_profiles_generate,ce_profiles_generate,gametypes_generate}.go` | Generate-on-save + the deferred CE stamp. Integration-tested. |
| `internal/pocketbase/routes/lansaves/{identity,serve}.go` | Per-gamertag manifest + file streaming. |
| `sveltekit/src/routes/gamertag/` | The side-by-side CE + H2 profile editor (RequireAuth). |
| `sveltekit/src/routes/organizer/{gametypes,games}/` | Organizer-gated library + uploads. |
| `sveltekit/src/lib/components/gamertag/` | `H2AppearanceEditor`, `CESettingsEditor`, `GametypeForm`, `SaveResultCard`. |
| `sveltekit/src/lib/{utils,types}/gamertag.ts` | FE helpers + record types. |

## Verification

- ✅ Go: `go build`, `go vet`, `go test` (saveartifact unit + hooks
  integration: create/update generate a valid tar, rename regenerates, bad input
  rejects the save, CE defers).
- ✅ Frontend: `pnpm check` (0 errors), `pnpm lint`, `pnpm test` (240), `pnpm build`.
- ✅ Live server (isolated port): all four collections register; creating a CE
  gametype + H2 profile via the API generates real save bundles; the identity
  manifest resolves a gamertag; `/file/...` streams a valid `application/x-tar`;
  the UI editors render and save with zero console errors.

## Open / deferred

1. **CE profile generation** — pending the CE player-name/profile research. Seam ready.
2. **Digest enforcement** — inherited LAN-hub open item (live-xemu probe).
3. **H2 field maps** — provisional labels; more samples would finish them.
4. **nxdk client** — wire the two per-gamertag endpoints + on-box unpack.
