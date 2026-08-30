# Deployments

How cartographer is deployed, under the host's **/srv Hosting Standard**
(`/srv/shared/HOSTING-PLAN.md`; canonical registries in `/srv/registry/` —
[`SITES.md`](/srv/registry/SITES.md) + [`PORTS.md`](/srv/registry/PORTS.md)).
Three tiers. Deployed tiers hold **artifacts + state only** (binary, `pb_data/`,
`.env`) — never a git checkout; builds happen in the repo, installs land under
`/srv`.

> **Cartographer runs NATIVELY, not in Docker.** It provisions xemu containers
> and needs host podman, `/dev/kvm`, `/dev/dri` and raw sockets, so each tier is
> a plain binary with its own run dir, `.env`, port and `pb_data`.

## Tiers

| Tier | Purpose | Runs from | How | Port | Hostname | Bot |
| --- | --- | --- | --- | --- | --- | --- |
| **dev** | live-reload coding | this working tree (Air + Vite HMR) | `./run-dev.sh` | `19099` vite / `19090` backend | `lab.norcal.pro` (only while up) | **off** (forced) |
| **pre** (test) | the gate before prod — merged branches soak here | `/srv/http/xemu-cartographer/pre/` | `/srv/registry/srv-pre.sh xemu-cartographer [ref]` → systemd **user** unit `site-xemu-cartographer-pre` | `18099` | `beta.norcal.pro` | cart-beta **test** app (per tier `.env`) |
| **prod** | the live site | `/srv/http/xemu-cartographer/prod/` | systemd unit `site-xemu-cartographer-prod` (migrated 2026-08-27; still root + `0.0.0.0` — de-root/loopback is a pending follow-up) | `8099` | `lan.norcal.pro` | cart **prod** app |

- Ports are claimed in the **canonical host registry**
  `/srv/registry/PORTS.md` (cartographer's spread — `8099`, `18099`,
  `19090/19099`, rig reservations `3300-3399` tcp + `9970-9989` udp — is
  grandfathered; don't renumber). Audit with `sudo /srv/registry/check-ports.sh`.
- Tiers bind **loopback only** (prod's `0.0.0.0` is the known exception being
  fixed); cloudflared is the public front door, the LAN Caddy serves
  `lan.local`.
- Data is separate per tier: each run dir has its own `pb_data/`. Dev's is
  ephemeral (`tmp/pb_data`, wiped on exit).
- **Discord: one gateway per bot token** — each tier that runs the bot needs its
  own Discord application; dev never runs it.
- Naming hazard: prod `lan.norcal.pro` and dev `lab.norcal.pro` are one letter
  apart in the same ingress list — double-check which you're editing.

## Promotion path

```
dev (working tree)  ──►  pre (/srv, :18099)  ──►  prod (/srv, :8099)
   run-dev.sh            srv-pre.sh               site-xemu-cartographer-prod
```

Nothing should reach prod without soaking on pre. For anything touching the
schema, pre applies the migration first — see [MIGRATIONS.md](MIGRATIONS.md).

## Deploying pre

```sh
/srv/registry/srv-pre.sh xemu-cartographer beta     # build the beta branch tip
/srv/registry/srv-pre.sh xemu-cartographer <ref>    # any committish
/srv/registry/srv-pre.sh xemu-cartographer status|logs|restart|stop|info
```

What it does (see the script header for the full contract): builds the ref in a
**temporary detached worktree** (only committed code ships — the repo checkout
is never touched), installs `bin/server` + `pb_public/` + `tools/game-maps/`
into the tier, regenerates `run.sh`, writes `BUILD-INFO` with provenance
verification, then (re)starts the `site-xemu-cartographer-pre` **user** unit
(linger is on — survives reboots, no sudo) and polls `/api/health`. It never
touches `.env` or `pb_data/`; a missing `.env` is seeded once from the repo's
`.env.pre` / `*.example` and must be reviewed.

**Pending migrations apply on boot**, so a healthy check also proves the
migrations applied.

## Rollback

```sh
/srv/registry/srv-pre.sh xemu-cartographer <last-good-ref>
```

⚠️ Code rolls back; **migrations do not**. An applied migration stays applied —
that's why they're proven on pre first. To undo a schema change, write a new
forward migration.

## Backups

`/srv/backups/` + a nightly snapshot timer is a planned phase of the hosting
standard (not live yet). Until then, copy a tier's `pb_data` yourself before
risky migrations: `cp -r <tier>/pb_data <tier>/pb_data.bak-$(date +%Y%m%d)` —
and prune old copies periodically; they are full copies.

## History

The pre-/srv deployment generations are retired: `deploy-beta.sh` (in-repo
full-cycle wrapper), then `~/xcarto-beta` + `pull-beta.sh` (home-dir run dir,
manual start), then briefly a hand-rolled `/srv/.../pre` on an unregistered
port. All superseded by `srv-pre.sh` + systemd units per the hosting standard.
