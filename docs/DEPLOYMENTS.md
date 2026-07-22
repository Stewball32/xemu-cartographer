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
| **beta** (test) | the gate before prod | built snapshot in `~/xcarto-beta` | `./deploy-beta.sh` | `~/xcarto-beta/.env` | cart-beta **test** app |
| **prod** | the live site | built snapshot in `/var/lib/xemu-cartographer` | *(manual — not yet scripted)* | prod `.env` | cart **prod** app |

## This project

| Tier | Host port | Public hostname | Branch |
| --- | --- | --- | --- |
| dev (vite) | `19099` | `dev.norcal.pro` | any |
| dev (backend) | `19090` | _internal — proxied by Vite_ | any |
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
dev (working tree)  ──►  beta (test)  ──►  prod
   run-dev.sh            deploy-beta.sh     (manual)
```

Nothing should reach prod without passing through beta. For anything touching the
schema, beta applies the migration first — see [MIGRATIONS.md](MIGRATIONS.md).

## Deploying

```sh
./deploy-beta.sh           # guard → build → deploy → health-check
./deploy-beta.sh logs      # follow logs
./deploy-beta.sh down      # stop the tier (keeps pb_data)
```

**Guards.** It refuses to run unless you're on the tier's branch with a clean
tree — a deploy builds from the working tree, so a dirty tree ships something
that isn't committed and can't be reproduced or rolled back to. Override
deliberately:

```sh
ALLOW_DIRTY=1 ./deploy-beta.sh          # uncommitted changes
ALLOW_ANY_BRANCH=1 ./deploy-beta.sh     # different branch
SKIP_BACKUP=1 ./deploy-beta.sh          # don't snapshot pb_data first
BETA_DIR=/some/other/dir ./deploy-beta.sh
```

**What a deploy does:**

1. Guards: on branch `beta`, clean tree, required commands/files present.
2. Backs up the tier's `pb_data` (timestamped, in the run dir).
3. Stops the running tier **by PID** (looked up from the listening port — never
   `pkill -f`, whose pattern can match and kill the deploying shell itself).
4. Builds the static frontend (`PUBLIC_PB_PORT` from the tier `.env`) and the
   production Go binary (**no `-tags dev`**, so Automigrate stays off).
5. Copies that snapshot (`server` + `pb_public`) into the run dir — `pb_data` is
   untouched.
6. Starts `run-beta.sh`, which sources the tier `.env` and execs the binary.
7. Polls `/api/health` for ~60s, dumping logs and failing if it never comes up.
   **Pending migrations apply on boot, so a healthy check also proves the
   migrations applied.**

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
./deploy-beta.sh            # rebuilds and redeploys that revision
```

⚠️ Code rolls back; **migrations do not**. An applied migration stays applied —
that's why they're proven on beta first. To undo a schema change, write a new
forward migration.

## Backups

`deploy-beta.sh` snapshots `pb_data` to `pb_data.bak-<timestamp>` in the run dir
before every deploy (skip with `SKIP_BACKUP=1`). Prune old ones periodically —
they are full copies.
