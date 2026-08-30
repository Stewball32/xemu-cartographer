# Deployments

How cartographer is deployed. Three tiers, one interface.

> **Cartographer runs NATIVELY, not in Docker.** It provisions xemu containers
> and needs host podman, `/dev/kvm`, `/dev/dri` and raw sockets, so each tier is
> a plain process with its own run dir, `.env`, port and `pb_data` — rather than
> a compose stack. The *interface* (guard → build → deploy → health-check, the
> same flags and guards) matches the [site-template standard](https://github.com/)
> so every project deploys the same way.

## Tiers

| Tier | Purpose | Runs from | Command | Env | Bot |
| --- | --- | --- | --- | --- | --- |
| **dev** | live-reload coding | this working tree (Air + Vite HMR) | `./run-dev.sh` | `.env.dev` | **off** (forced) |
| **pre** (preview) | merged-branch testing before beta | built snapshot in `/srv/http/xemu-cartographer/pre` | `BETA_DIR=/srv/http/xemu-cartographer/pre ~/xcarto-beta/pull-beta.sh` | `<run-dir>/.env` | off |
| **beta** (test) | the gate before prod | built snapshot in `~/xcarto-beta` | `~/xcarto-beta/pull-beta.sh`, then start manually | `~/xcarto-beta/.env` | cart-beta **test** app |
| **prod** | the live site | built snapshot in `/var/lib/xemu-cartographer` | *(manual — not yet scripted)* | prod `.env` | cart **prod** app |

## This project

| Tier | Host port | Public hostname | Branch |
| --- | --- | --- | --- |
| dev (vite) | `19099` | `dev.norcal.pro` | any |
| dev (backend) | `19090` | _internal — proxied by Vite_ | any |
| pre (preview) | `17099` | _loopback only (no tunnel yet)_ | `beta` |
| beta (test) | `18099` | `beta.norcal.pro` | `beta` |
| prod | `8099` | `lan.norcal.pro` | `main` |

- Ports come from cartographer's row in [`../PORTS.md`](../PORTS.md) (the
  grandfathered, interleaved block — **do not** renumber; `8098`–`8101` is shared
  with norcal-halo-site). Provisioned xemu containers use base `3300` on beta.
- Every tier binds **loopback only**; the cloudflared tunnel is the only way in.
- Data is **separate per tier**: prod `/var/lib/xemu-cartographer/pb_data`, beta
  `~/xcarto-beta/pb_data`, dev `./tmp-dev/pb_data` (ephemeral, wiped on exit).
- **Discord: one gateway per bot token.** Each tier that runs the bot needs its
  OWN Discord application — beta uses the `cart-beta` test app; dev never runs it.

## Promotion path

```
dev (working tree)  ──►  pre / beta (test)  ──►  prod
   run-dev.sh            pull-beta.sh           (manual)
```

Nothing should reach prod without passing through beta. For anything touching the
schema, beta applies the migration first — see [MIGRATIONS.md](MIGRATIONS.md).

## Deploying (pull-beta.sh — build + install, start yourself)

The old `deploy-beta.sh` full-cycle wrapper (stop → deploy → start →
health-check) is retired; the tier owner starts the process. `pull-beta.sh`
lives IN the run dir (`~/xcarto-beta`), builds from this repo, and installs
into the dir it lives in — or any dir via `BETA_DIR=`:

```sh
~/xcarto-beta/pull-beta.sh              # build local `beta` → install into ~/xcarto-beta
~/xcarto-beta/pull-beta.sh --fetch      # fast-forward from origin/beta first
BETA_DIR=/srv/http/xemu-cartographer/pre ~/xcarto-beta/pull-beta.sh   # the pre tier
```

**What it does:**

1. Guards: on branch `beta` (`BETA_BRANCH=`/`ALLOW_ANY_BRANCH=1` to override),
   the target's `.env` present, the tier's port NOT live (it refuses to
   overwrite the binary of a running process — the kernel keeps the old inode
   mapped and a later restart is what actually changes behaviour).
2. Builds the static frontend (`PUBLIC_PB_PORT` from the target `.env`) and the
   production Go binary (**no `-tags dev`**, so Automigrate + seeding stay off).
3. Installs `server`, `pb_public/`, and `tools/game-maps/` (the thumbnail
   renderer) into the run dir; regenerates the run script if missing; writes
   `BUILD-INFO` and verifies the installed binary's `vcs.revision` against
   HEAD. `.env`, `pb_data/`, `containers/`, `inbox/` are never touched.
4. Hands off. Start it yourself (`sudo ./run-beta.sh`, foreground or nohup) and
   verify `/api/health`. **Pending migrations apply on boot**, so a healthy
   boot also proves the migrations applied.

## First-time setup (a new tier)

1. Confirm the port block in [`../PORTS.md`](../PORTS.md).
2. Create the run dir; `cp .env.dev.example <run-dir>/.env` and fill it in
   (`chmod 600`). Give the tier its **own** `SEED_SUPERUSER_*` — never prod's.
3. If the bot runs on this tier: a **separate Discord application** + token.
4. Add the cloudflared ingress rule + DNS record for the hostname.
5. Deploy, then verify the health check and the logs.

## Rollback

```sh
git checkout <last-good-sha>
~/xcarto-beta/pull-beta.sh   # rebuilds + reinstalls that revision (tier stopped)
```

⚠️ Code rolls back; **migrations do not**. An applied migration stays applied —
that's why they're proven on beta first. To undo a schema change, write a new
forward migration.

## Backups

`pull-beta.sh` does **not** snapshot `pb_data` (the retired deploy-beta.sh
did). Copy `pb_data` yourself before risky migrations:
`cp -r <run-dir>/pb_data <run-dir>/pb_data.bak-$(date +%Y%m%d)` — and prune old
ones periodically; they are full copies.
