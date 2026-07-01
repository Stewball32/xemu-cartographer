# Gamertag identity system

> **Status:** Built + live-verified (2026-06-25, branch `feat/gamertag-system`).
> The **H2 profile** generates the full appearance/controls `profile`,
> **correctly signed** (the halosave 20-byte digest is implemented + verified
> in-game). The **CE profile** is a full profile too (CE has its own player
> profiles, format cracked) — generates a signed `blam.sav` (name + armor color +
> control presets). A gamertag generates ONLY these two player profiles — it does
> **not** set the Xbox console name (that's a per-console setting, unrelated to a
> user's identity).

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
only — the shorter of the two games' name fields: CE's `blam.sav` name is a
24-byte UTF-16LE buffer ⇒ ≤11 chars (the controlling limit); H2 holds ~113. The
frontend mirrors the number (`GAMERTAG_MAX_LEN`).

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
| `ce_profiles` | `user` (unique), `settings` (json: color/thumbstick/button), `save_bundle` (file), `save_info` (json) — gamertag read from the user | yes — signed `blam.sav` + `SaveMeta.xbx`, tar'd | owner read/write; admin anything |
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
- `ce_profiles` generates a **signed `blam.sav`** (name + armor color + control
  presets) via `internal/halosave` `CEProfileBuild`, signed with the same
  per-title HMAC as the CE gametype (digest at 0x30).
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

## The CE profile (signed `blam.sav`)

**Halo: CE has its own player profile** on Xbox — `blam.sav` (512 B), parallel
to H2, format cracked (see [CE-PROFILE-FORMAT.md](CE-PROFILE-FORMAT.md)). Stored
at `E:\UDATA\4d530004\<12-hex>\{blam.sav, SaveMeta.xbx}` (+ a CE-auto-created
`savegame.bin`). Signed region `[0:0x30]`:

| offset | field |
| --- | --- |
| `0x00` | name (UTF-16LE, 24-byte buf, ≤11 chars) = the gamertag |
| `0x18` | u32 armor color enum (white=0, red=2, blue=3) |
| `0x28`/`0x29` | thumbstick / button preset (0=Default, 1=Southpaw) |
| `0x1C-0x2F` | advanced bytes (sensitivity/invert/vibration — pluggable follow-up) |
| `0x30` | 20-byte HMAC-SHA1 digest over `[0:0x30]` |

- `ce_profiles` `settings` holds **color / thumbstick / button**; the generate
  hook builds the signed `blam.sav` via `saveartifact.CEProfileRequest` →
  `halosave.CEProfileBuild`, re-signed at 0x30 with the **same per-title HMAC**
  as the CE gametype (`RecomputeDigest(buf,0x30,20,"ce")`). Verified byte-exact
  against a real captured profile.
- The CE editor (`/gamertag/`, Halo: CE column) exposes color + the two control
  presets; the advanced `0x1C-0x2F` bytes are a follow-up (one more xemu capture).
- Open: whether a stub `savegame.bin` must ship for CE to list the profile (CE
  auto-creates it; the hub currently ships `blam.sav` + `SaveMeta.xbx`).

> A gamertag generates **only** the CE + H2 player profiles. It does **not** set
> the Xbox console name (`NICKNAME.XBN`) — that is a per-console setting, not part
> of a user's identity. (The fleet's M26 auto-naming of xemu *instances* via
> `internal/podman` `console_name.go` is unrelated and unchanged.)

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
| `internal/halosave` | `CEProfileBuild`/`CEProfileParse` (signed `blam.sav`) + H2/gametype builders + the per-title HMAC digest. Unit-tested (CE signing verified byte-exact vs a real sample). |
| `internal/saveartifact` | Pure: typed settings → `halosave.BuildRequest` → SaveSet + tar bundle + lean `Info`; `CEProfileRequest`. Unit-tested. |
| `internal/pocketbase/schema/{ce_profiles,h2_profiles,gametypes,game_titles}.go` | The four collections + access rules. |
| `internal/pocketbase/schema/{roles,rules,identity,users}.go` | `organizer` role, `organizerOrAdmin` rule, `users.gamertag` field + cap, phase-5 registration. |
| `internal/pocketbase/hooks/{save_artifact,h2_profiles_generate,ce_profiles_generate,gametypes_generate,users_gamertag_regen}.go` | Generate-on-save (CE signed `blam.sav`, H2 signed profile, gametypes) + gamertag-change cascade. Integration-tested. |
| `internal/pocketbase/routes/lansaves/{identity,serve}.go` | Per-gamertag manifest + file streaming. |
| `sveltekit/src/routes/gamertag/` | The side-by-side CE + H2 profile editor (RequireAuth). |
| `sveltekit/src/routes/organizer/{gametypes,games}/` | Organizer-gated library + uploads. |
| `sveltekit/src/lib/components/gamertag/` | `H2AppearanceEditor`, `CESettingsEditor` (CE color + control presets), `GametypeForm`, `SaveResultCard`. |
| `sveltekit/src/lib/{utils,types}/gamertag.ts` | FE helpers + record types. |

## Verification

- ✅ Go: `go build`, `go vet`, `go test` (halosave: CE `blam.sav` signing is
  byte-exact vs a real captured profile; saveartifact unit; hooks integration:
  CE + H2 generate signed bundles, gametypes generate, the gamertag-rename
  cascade regenerates, bad input rejects).
- ✅ Frontend: `pnpm check` (0 errors), `pnpm lint`, `pnpm test` (240), `pnpm build`.
- ✅ Live server (isolated port): the 11-char + printable-ASCII cap is enforced;
  setting `users.gamertag` then creating the CE profile generates a **signed
  `blam.sav`** (correct name/color/presets; HMAC over `[0:0x30]` matches the
  stored digest) and the H2 profile a signed bundle; the manifest resolves a
  gamertag to both profiles; the UI editors render and save with zero console
  errors.

## Open / deferred

1. **CE advanced bytes** (`0x1C-0x2F`: sensitivity/invert/vibration) — not yet
   individually mapped; one more xemu capture pins them down. Defaults applied.
2. **CE `savegame.bin`** — confirm whether a stub campaign save must ship for CE
   to list a generated profile (CE auto-creates it).
3. **H2 field maps** — provisional appearance labels; more samples would finish them.
4. **nxdk client** — wire the per-gamertag endpoints + on-box unpack.

(The digest/signing and CE-profile-format open items are **closed** — both CE
`blam.sav` and H2 `profile` are now signed + generated.)
