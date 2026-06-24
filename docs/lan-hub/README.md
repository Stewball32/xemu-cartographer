# LAN hub — Halo save generator & download API

> **Status:** Working (offline-verified). The one open item is a runtime check on
> a live xemu (does Halo enforce the 20-byte save digest?), de-risked to "yes,
> with a known finishing step." See [Open items](#open-items).

The **file-based** half of the LAN Xbox-management system: the cartographer hub
**generates Xbox-acceptable Halo: CE / Halo 2 save files** (multiplayer gametype
variants and Halo 2 player profiles) and **serves them to a LAN client** (the
de-risked nxdk download client) over HTTP, with a disk-space pre-flight, to be
written onto the Xbox's FATX disk.

This is the complement to [M24](../milestones/M24-remote-game-setup.md)'s
**memory-write** approach. M24 pokes a *running* game's lobby state; this writes
the *on-disk* UDATA saves the game reads at boot. They are independent and can
ship separately.

```
 Browser editor (admin)                       nxdk client (Xbox, LAN)
        │  POST /api/lan/saves/build                 │  GET /api/lan/saves/download?…&free_bytes=N
        │  (live preview + round-trip validate)      │  (disk-space pre-flight → file/bundle)
        ▼                                            ▼
 ┌─────────────────────────────────────────────────────────────┐
 │  internal/pocketbase/routes/lansaves   (HTTP, disk check)    │
 │  internal/halosave                     (template-patch gen)  │
 │  internal/diskspace                    (statfs + FATX footprint) │
 └─────────────────────────────────────────────────────────────┘
            writes →  E:\UDATA\<titleID>\<dir>\{blam.lst|profile|slayer, SaveMeta.xbx}
```

## How it works — template-patch

The save **content** formats are cracked (see
[reference/FORMATS.md](reference/FORMATS.md)), but every payload save carries a
**20-byte content-dependent integrity digest** whose algorithm is not yet
recovered offline. So the generator never builds a file from nothing: it takes a
real, same-engine save as a **template**, overwrites only the fields we
understand, and preserves every other byte — including the digest and the
trailing padding.

- Build with **no field changes** → **byte-identical** to a real save.
- Build with **edited settings** → identical except the patched bytes (the digest
  is **preserved**, i.e. stale relative to the new content).

The `internal/halosave` Go package is a faithful port of the de-risk's Python
generator (`reference/halo_save_formats.py`). It is verified two ways: a Go
round-trip suite that re-serializes **all 23 CE + 3 H2 real samples
byte-identically**, and a live cross-check confirming the HTTP endpoint produces
output **byte-identical (matching SHA-1) to the Python reference generator** for
CE gametype, H2 profile, and H2 gametype.

### Covered file types

| Title | Kind | Payload file | Size | Mapped fields |
| --- | --- | --- | --- | --- |
| Halo: CE | gametype | `blam.lst` | 512 B | name, engine, teams, options, scoring subtype, time limit(s), score limit, option2, respawn, engine union |
| Halo 2 | profile | `profile` | 500 B | player name, appearance/controller bytes (provisional) |
| Halo 2 | gametype | `<mode>` (e.g. `slayer`) | var | name, score limit (partial map — 1 sample) |
| both | — | `SaveMeta.xbx` | tiny | display name (fully cracked, **unsigned**) |

**Halo: CE has no standalone multiplayer player-profile save** — the editable CE
surface is the gametype only. The appearance/controls editor is Halo 2 only.

CE engine templates (one clean real sample per engine) are embedded for:
`slayer`, `ctf`, `oddball`, `king`, `race`. Switching engine in the editor swaps
the underlying template.

## API

Base path `/api/lan/saves`. Access policy (`authorizeLAN`): an authenticated
admin is always allowed; if `LAN_SAVES_TOKEN` is set, a request presenting it
(`X-LAN-Token` header or `?token=`) is allowed; if it is unset the endpoints are
**open** (the trusted-LAN default). `Authorization` is reserved for the PB JWT.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/meta` | Generator capabilities (titles/kinds, CE engines, H2 appearance fields, digest status) for the editor UI |
| GET/POST | `/build` | Preview + **round-trip validate** a spec — returns metadata (file names/sizes/sha1, re-parsed fields, digest status, footprint). No file bytes. |
| GET/POST | `/download` | Generate and **serve** the save, gated by a disk-space pre-flight |

### Build a save (spec)

The flat spec is shared by `/build` and `/download` (JSON body on POST, query
params on GET). Only the fields you set are changed; everything else is the
template's value.

```
title=ce|h2   kind=gametype|profile   name=<variant/player name>
engine=slayer|ctf|oddball|king|race                  (CE)
teams=1|0   radar=1|0   score_limit=N   time_minutes=N (CE friendly)
options=0x22  scoring_subtype  time_limit  time_limit2  option2  respawn  engine_union (CE raw)
app_<key>=N                                          (H2 appearance bytes, 0–255)
internal_name=…   dir_name=…                         (optional overrides)
```

GET example (the nxdk client URL):

```
GET /api/lan/saves/download?title=ce&kind=gametype&engine=slayer&name=TS+25
      &score_limit=25&time_minutes=7&free_bytes=33554432&format=tar
```

### Disk-space check

The endpoint computes the **FATX on-disk footprint** of the whole save directory
(each file rounded up to a 16 KiB cluster + one cluster for the dir) and checks
it before serving:

1. **Target (Xbox):** the client reports its free FATX bytes via `free_bytes`. If
   the footprint won't fit → **`507 Insufficient Storage`** with a JSON body
   (`footprint_bytes`, `available_bytes`, `fatx_cluster`, `fatx_dir`). This is
   the meaningful pre-flight — a half-written save corrupts the FATX directory.
2. **Server staging:** the server also `statfs`-checks its own staging dir
   (`LAN_SAVES_STAGING_DIR`, default `$TMPDIR`) so it never generates into a
   server that is itself full. Best-effort (a statfs error is logged, not fatal).

`cluster_size` and `LAN_SAVES_FATX_CLUSTER` override the cluster assumption.

### Download formats (`format=`)

| `format` | Body |
| --- | --- |
| `tar` (default) | tar of the save dir at `UDATA/<titleID>/<dir>/…` (unpack relative to `E:\`) |
| `zip` | same layout, zip (human-friendly) |
| `payload` | just the payload file (`blam.lst` / `profile` / `<mode>`) |
| `savemeta` | just `SaveMeta.xbx` |
| `file` + `file=<name>` | a specific named file |

Response headers carry placement + digest metadata for the client:
`X-Fatx-Dir`, `X-Fatx-Footprint-Bytes`, `X-Save-Title`, `X-Save-Kind`,
`X-Digest-Mode`, `X-Digest-Edited`, `X-Digest-Resolved`, `Content-Disposition`.

## The 20-byte digest — the one open item

Each payload save (CE `blam.lst` @`0x68`; H2 trailing) carries a 20-byte,
content-dependent digest. It is **not** a plain SHA-1/MD5/CRC of any contiguous
range, and **not** HMAC-SHA1 under obvious keys (tested exhaustively) ⇒ a
keyed/salted digest (almost certainly the Xbox content signature under a
per-title key from the Halo XBE, or a Halo-internal salted SHA-1). See
[reference/FORMATS.md §6](reference/FORMATS.md) and
[reference/DE-RISK-VERDICT.md](reference/DE-RISK-VERDICT.md).

**The clean hook is wired and ready.** `halosave.RecomputeDigest(buf, off, len,
title)` is the single seam; it currently returns `ErrDigestUnresolved`, and
`Build*` defaults to template-patch (`recompute=false`). When the algorithm/key
is recovered, implement that one function and pass `recompute=true`. `DigestResolved()`
flips automatically and the API/UI surface it. **No other code changes.**

The open question that actually gates *edited-settings* files is **whether Halo
re-verifies the digest on load** — many Xbox titles write an internal save hash
but never check it. That is a ~10-minute live-xemu probe (procedure in the
de-risk verdict). If unenforced, template-patch is already fully accepted and
this item closes with zero further work. If enforced, the preferred path is to
drive the in-game menu with the existing pad-fleet automation so the engine
writes a correct digest natively (sidesteps the crack).

## Code map

| Package / path | Role |
| --- | --- |
| [`internal/halosave`](../../internal/halosave) | Parser + template-patch generator (CE gametype, H2 profile, H2 gametype, SaveMeta), embedded templates, digest hook. Unit-tested against all real samples. |
| [`internal/diskspace`](../../internal/diskspace) | `statfs` free-space + FATX cluster footprint estimate. Unit-tested. |
| [`internal/pocketbase/routes/lansaves`](../../internal/pocketbase/routes/lansaves) | `/api/lan/saves/*` — meta / build / download + `authorizeLAN` + archive helpers. |
| `sveltekit/src/routes/admin/lan/` | The editor page (tabs: CE gametype / H2 profile / H2 gametype). |
| `sveltekit/src/lib/components/lan/` | `CEGametypeEditor`, `H2ProfileEditor`, `H2GametypeEditor`, `SavePreview`. |
| `sveltekit/src/lib/utils/lansaves.ts` | Front-end client (`lanMeta` / `lanBuild` / `lanDownload` / `lanDownloadURL`). |
| [`reference/`](reference) | The de-risk deliverable verbatim: format spec, verdict, Python reference generator + round-trip test + FATX extractor. |

## Configuration (env)

| Var | Default | Meaning |
| --- | --- | --- |
| `LAN_SAVES_TOKEN` | _(unset → open)_ | Shared token gating the LAN endpoints (admin JWT always works) |
| `LAN_SAVES_STAGING_DIR` | `$TMPDIR` | Dir whose free space the server checks before generating |
| `LAN_SAVES_FATX_CLUSTER` | `16384` | FATX cluster size for the footprint estimate |

## Verification status

- ✅ Go generator: round-trips **all 23 CE + 3 H2 real samples byte-identically**.
- ✅ HTTP endpoint output is **byte-identical (matching SHA-1)** to the Python
  reference generator for CE gametype, H2 profile, and H2 gametype.
- ✅ Disk-space check: `507` when the target can't hold the footprint; `200`
  otherwise; FATX/digest response headers verified.
- ✅ `go test ./...`, `go vet ./...`, `pnpm check`, `pnpm lint`, `pnpm build`,
  `pnpm test` all pass.
- ⏳ **Live xemu:** confirm Halo accepts an edited-settings file (digest
  enforcement). Deferred per the no-xemu constraint; exact procedure in the
  de-risk verdict.

## Open items

1. **Digest enforcement probe** (live xemu) — closes the edited-file question.
2. **Recompute the digest** only if step 1 shows it is enforced — or use the
   menu-drive fallback. Hook is ready.
3. **H2 field maps** are partial (1 gametype, 2 profiles). Capturing a few more
   H2 saves and diffing would finish labelling — mechanical.
4. **nxdk client wiring** — the client GETs `/download` with its FATX free bytes;
   verify against the real device.
