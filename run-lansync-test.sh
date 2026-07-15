#!/usr/bin/env bash
# run-lansync-test.sh — boot a standalone LAN-sync TEST server (feat/lan-sync).
#
# Compiles with -tags dev so the in-process seeder runs, and sets
# SEED_LAN_SYNC=true so it also seeds the full LAN box-provisioning E2E scenario
# (1 lan_event, 3 checked-in players with generated CE+H2 profiles, 1 extractable
# ISO, 1 app zip, 1 ACTIVE preset). ISO bank + extracted cache live under /tmp so
# the working tree stays clean. Fully separate from prod + beta + `task dev`.
#
# Binds to the LAN (0.0.0.0) so a xemu instance or a real Xbox on the network can
# reach it — the on-Xbox sync client hits the printed LAN manifest URL.
#
# Usage:  ./run-lansync-test.sh                 # foreground; Ctrl-C to stop
#         PORT=8790 ./run-lansync-test.sh       # override the port
#         BIND=127.0.0.1 ./run-lansync-test.sh  # loopback-only (old behavior)
#         LAN_TOKEN=secret ./run-lansync-test.sh # lock down; client sends the token
set -euo pipefail

PORT="${PORT:-8790}"
BIND="${BIND:-0.0.0.0}"                       # LAN-reachable by default
ROOT="${LANSYNC_TEST_ROOT:-/tmp/xcarto-lansync-test}"
# LAN_TOKEN is the authorizeLAN shared token (env var LAN_SAVES_TOKEN). Unset =
# OPEN on the LAN (the trusted-appliance default) — behavior unchanged.
LAN_TOKEN="${LAN_TOKEN:-${LAN_SAVES_TOKEN:-}}"

mkdir -p "$ROOT/isos" "$ROOT/extracted" "$ROOT/pb_data"

# Primary LAN IP (the src address of the default route); fall back to hostname -I.
LAN_IP="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}')"
[ -n "$LAN_IP" ] || LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -n "$LAN_IP" ] || LAN_IP="127.0.0.1"

echo "LAN-sync test server"
echo "  data dir : $ROOT/pb_data (ephemeral — delete to reset)"
echo "  iso bank : $ROOT/isos"
echo "  extracted: $ROOT/extracted"
echo "  bind     : $BIND:$PORT"
echo "  local    : http://127.0.0.1:$PORT/api/lan/sync/manifest?preset=active"
echo "  LAN URL  : http://$LAN_IP:$PORT/api/lan/sync/manifest?preset=active"
echo "             ^ point the Xbox sync client's server_host at $LAN_IP:$PORT"
if [ -n "$LAN_TOKEN" ]; then
  echo "  auth     : LAN token REQUIRED — client sends header 'X-LAN-Token: $LAN_TOKEN'"
  echo "             or appends '&token=$LAN_TOKEN' to the URL"
else
  echo "  auth     : OPEN on the LAN (no token). To require one, re-run with"
  echo "             LAN_TOKEN=<secret> ./run-lansync-test.sh"
fi
echo

# Requires extract-xiso on PATH (the seed builds + extracts a placeholder XISO).
command -v extract-xiso >/dev/null || {
  echo "WARNING: extract-xiso not found on PATH — the ISO seed step will fail." >&2
}

exec env \
  SEED_LAN_SYNC=true \
  CONTAINERS_ISO_DIR="$ROOT/isos" \
  LAN_SYNC_EXTRACT_DIR="$ROOT/extracted" \
  LAN_SAVES_TOKEN="$LAN_TOKEN" \
  go run -tags dev ./cmd/server serve --dir "$ROOT/pb_data" --http "$BIND:$PORT"
