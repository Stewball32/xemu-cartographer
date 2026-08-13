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

# --- DEV environment (.env.dev) ----------------------------------------------
# Tier env lives in ./.env.dev (gitignored) — see .env.dev.example. Same pattern
# as the beta tier's .env, and the same interface as the site template.
ENV_FILE="$PWD/.env.dev"
if [ ! -f "$ENV_FILE" ]; then
	echo "run-dev.sh: $ENV_FILE not found — copy .env.dev.example to .env.dev and fill it in." >&2
	exit 1
fi
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

# NO DISCORD BOT on dev — HARD REQUIREMENT, enforced HERE (after sourcing) so it
# holds no matter what .env.dev or the launching shell contains. The dev server
# restarts constantly (Air rebuilds on every Go edit); each restart would re-open
# the gateway and re-register commands, which Discord rate-limits, and would
# fight prod's bot (one gateway per token). OAuth-only.
unset DISCORD_BOT_TOKEN
unset DISCORD_DEV_GUILD_ID

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
