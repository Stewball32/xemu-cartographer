#!/usr/bin/env bash
# DEV tier — dev.norcal.pro — the live-reload-from-repo layer.
#
# Runs from THIS repo working tree (whatever branch is checked out): a hot-reload
# PocketBase (Air) + the SvelteKit dev server (Vite HMR). Code edits show
# instantly. Distinct from prod (:8099, /var/lib) and beta (:18099, built
# snapshot).
#
#   dev PocketBase : http://127.0.0.1:19090   (Air, -tags dev, ephemeral pb_data)
#   Vite dev       : http://0.0.0.0:19099      (HMR; proxies /api + /_ → dev PB)
#   public         : https://dev.norcal.pro    (cloudflared → localhost:19099)
#
# Start:  ./run-dev.sh              (foreground; Ctrl-C stops both)
#         nohup ./run-dev.sh >dev.log 2>&1 &
# Stop:   pkill -f '.air.dev.toml' ; pkill -f 'vite.config.dev'
set -uo pipefail
cd "$(dirname "$0")"

# --- DEV environment ---------------------------------------------------------

# NO DISCORD BOT on dev — HARD REQUIREMENT. The dev server restarts constantly
# (Air rebuilds on every Go edit); each restart re-opens the gateway and
# re-registers commands, which Discord rate-limits, and would fight prod's bot
# (one gateway per token). We UNSET it so dev can never inherit a token.
# OAuth-only (DISCORD_CLIENT_ID/SECRET may be set later for login testing).
unset DISCORD_BOT_TOKEN
unset DISCORD_DEV_GUILD_ID

# Containers OFF — dev is for code/UI iteration; don't contend with prod's podman.
export CONTAINERS_ENABLED=false

# Dev superuser: seeded on start (fresh ephemeral pb_data). The -tags dev seeder
# also creates a regular IsAdmin user admin@dev.com / admin123 (data.go).
export PB_SUPERUSER_EMAIL=root@dev.com
export PB_SUPERUSER_PASSWORD=root@dev.com

# Frontend build-time var (Vite reads it): the dev PB port. api-base.ts only
# uses it for localhost/LAN access; through dev.norcal.pro the API is proxied
# same-origin, so this value is irrelevant to the tunnel path.
export PUBLIC_PB_PORT=19090

# Overlay-token secret (stable so dev-minted tokens survive a restart).
export OVERLAY_TOKEN_SECRET="dev-only-not-a-real-secret-rotate-if-exposed"

DEV_PBDATA="./tmp-dev/pb_data"

cleanup() { kill "${AIR_PID:-}" "${VITE_PID:-}" 2>/dev/null; }
trap cleanup EXIT INT TERM

echo "[dev] starting PocketBase (Air) on :19090 ..."
air -c .air.dev.toml -- --http=127.0.0.1:19090 --dir="$DEV_PBDATA" &
AIR_PID=$!

# Wait for the dev PB to accept connections before starting Vite.
for i in $(seq 1 60); do
	if curl -sf -o /dev/null http://127.0.0.1:19090/api/health; then break; fi
	sleep 1
done

echo "[dev] starting Vite dev server on :19099 ..."
( cd sveltekit && exec pnpm exec vite dev --config vite.config.dev.ts ) &
VITE_PID=$!

wait
