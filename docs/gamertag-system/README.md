# Gamertag identity system

> **Status:** Built + live-verified (2026-06-25, branch `feat/gamertag-system`).
> One gamertag drives both profiles: the **CE profile** generates the Xbox
> console name `E:\UDATA\NICKNAME.XBN` (name-only — CE has no MP profile on
> Xbox), and the **H2 profile** generates the full appearance/controls `profile`,
> **correctly signed** (the halosave 20-byte digest is implemented + verified
> in-game — Halo 2 accepts the generated profiles).

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

It is length-capped to **`schema.GamertagMaxLen` (11 chars)**, printable-ASCII
only — the shorter of the two games' name fields (a gamertag must fit both).
CE's in-memory MP name buffer is 24 bytes ≈ 11 UTF-16 chars (the controlling,
tighter limit); H2's `profile` name field holds ~113. The frontend mirrors the
number (`GAMERTAG_MAX_LEN`).

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
| `users` (built-in) | + `gamertag` (text, ≤11, printable-ASCII) — the in-game name, separate from `username` | — | self read/write; admin |
| `ce_profiles` | `user` (unique), `save_bundle` (file), `save_info` (json) — name-only, gamertag read from the user | yes — `UDATA/NICKNAME.XBN` (console name), tar'd | owner read/write; admin anything |
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
- `ce_profiles` builds the **console name** `UDATA/NICKNAME.XBN` from the
  gamertag (`saveartifact.CEProfileBundle` → `internal/consolename`, the same
  builder the podman provisioner uses) — CE has no MP profile on Xbox, so the
  console name IS the CE profile (see below).
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

## The CE profile is name-only (the console name)

On the original Xbox, **Halo: CE has no MP player profile / appearance /
controls** (CE research, confirmed). The multiplayer name comes solely from the
Xbox console name `E:\UDATA\NICKNAME.XBN` (3400 bytes, plaintext, no checksum);
CE's MP color is an engine-assigned team slot (not saved), and look/sensitivity
are local per-controller (not per-identity, not networked). The "name + armor +
controls" profile people remember is **Halo PC, not Xbox CE**.

So the CE profile is name-only:

- `ce_profiles` has no editable fields — the generate hook builds `NICKNAME.XBN`
  from the user's gamertag via `saveartifact.CEProfileBundle` →
  [`internal/consolename`](../../internal/consolename) (the shared leaf package
  that `internal/podman` also uses to stamp a container's console name).
- The same gamertag therefore writes **both** `NICKNAME.XBN` (CE / dashboard /
  system-link name) and the H2 `profile`.
- NICKNAME.XBN is unsigned (plaintext), so there's no digest to recompute.

The CE editor (`/gamertag/`, Halo: CE column) is an info card: the gamertag is
the whole CE profile, and saving generates the console name.

## H2 customization field set

The H2 profile editor exposes the reverse-engineered appearance/controller bytes
(`halosave.H2ProfileFields`), grouped Armor / Emblem / Controls. Labels are
**provisional** (2-sample reverse engineering — surfaced as such in the UI).
More samples would finish labelling; the byte offsets are stable. H2 gametypes
map name + score limit (1 sample); other settings are preserved from the
template.

## The 20-byte digest — now signed

Every payload save (CE `blam.lst`, H2 `profile`/mode) carries a 20-byte
content-dependent HMAC digest. This is now **implemented and verified in-game**:
`halosave.RecomputeDigest` signs CE + H2, all builders **always re-sign**, and
the signature is **per-title (roamable)** — so the hub signs centrally and the
client needs no per-console signing. `saveartifact.Info.digest` reports
`mode: "recomputed"`, `resolved: true` for every generated file (the editor's
stale-digest warning is gated on `!resolved`, so it no longer fires).

This closes the LAN hub's one open item: edited-settings files are now correct,
not stale. (Earlier builds template-patched and preserved the template's digest,
which Halo 2 rejected as a "damaged profile" — the root cause this fix resolves.)

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
| `internal/consolename` | Pure leaf: the `NICKNAME.XBN` console-name format (`Sanitize`/`BuildXBN`). Shared by `internal/podman` (container naming) and the CE profile. Unit-tested. |
| `internal/saveartifact` | Pure: typed settings → `halosave.BuildRequest` → SaveSet + tar bundle + lean `Info`; plus `CEProfileBundle` (the NICKNAME.XBN tar). Unit-tested. |
| `internal/pocketbase/schema/{ce_profiles,h2_profiles,gametypes,game_titles}.go` | The four collections + access rules. |
| `internal/pocketbase/schema/{roles,rules,identity,users}.go` | `organizer` role, `organizerOrAdmin` rule, `users.gamertag` field + cap, phase-5 registration. |
| `internal/pocketbase/hooks/{save_artifact,h2_profiles_generate,ce_profiles_generate,gametypes_generate,users_gamertag_regen}.go` | Generate-on-save (CE NICKNAME.XBN, H2 signed profile, gametypes) + gamertag-change cascade. Integration-tested. |
| `internal/pocketbase/routes/lansaves/{identity,serve}.go` | Per-gamertag manifest + file streaming. |
| `sveltekit/src/routes/gamertag/` | The side-by-side CE + H2 profile editor (RequireAuth). |
| `sveltekit/src/routes/organizer/{gametypes,games}/` | Organizer-gated library + uploads. |
| `sveltekit/src/lib/components/gamertag/` | `H2AppearanceEditor`, `CESettingsEditor` (CE name-only info), `GametypeForm`, `SaveResultCard`. |
| `sveltekit/src/lib/{utils,types}/gamertag.ts` | FE helpers + record types. |

## Verification

- ✅ Go: `go build`, `go vet`, `go test` (consolename + saveartifact unit + hooks
  integration: CE profile generates `UDATA/NICKNAME.XBN`, H2 generates a signed
  profile, gametypes generate, the gamertag-rename cascade regenerates, bad input
  rejects the save).
- ✅ Frontend: `pnpm check` (0 errors), `pnpm lint`, `pnpm test` (240), `pnpm build`.
- ✅ Live server (isolated port): collections register; the 11-char + printable-
  ASCII gamertag cap is enforced; setting `users.gamertag` then creating profiles
  generates the CE `NICKNAME.XBN` (3400 B, correct header) **and** the signed H2
  profile; the manifest resolves a gamertag to both; `/file/...` streams a valid
  tar; the UI editors render and save with zero console errors.

## Open / deferred

1. **H2 field maps** — provisional appearance labels; more samples would finish them.
2. **nxdk client** — wire the two per-gamertag endpoints + on-box unpack.

(The digest/signing and CE-profile open items are **closed** — see "The 20-byte
digest" and "The CE profile is name-only" above.)
