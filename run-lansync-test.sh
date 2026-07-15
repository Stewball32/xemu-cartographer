#!/usr/bin/env bash
# run-lansync-test.sh — boot a standalone LAN-sync TEST server (feat/lan-sync).
#
# Compiles with -tags dev so the in-process seeder runs, and sets
# SEED_LAN_SYNC=true so it also seeds the full LAN box-provisioning E2E scenario
# (1 lan_event, 3 checked-in players with generated CE+H2 profiles, 1 extractable
# ISO, 1 app zip, 1 ACTIVE preset). ISO bank + extracted cache live under /tmp so
# the working tree stays clean. Fully separate from prod + beta + `task dev`.
#
# Usage:  ./run-lansync-test.sh            # foreground; Ctrl-C to stop
#         PORT=8790 ./run-lansync-test.sh  # override the port
set -euo pipefail

PORT="${PORT:-8790}"
ROOT="${LANSYNC_TEST_ROOT:-/tmp/xcarto-lansync-test}"

mkdir -p "$ROOT/isos" "$ROOT/extracted" "$ROOT/pb_data"

echo "LAN-sync test server"
echo "  data dir : $ROOT/pb_data (ephemeral — delete to reset)"
echo "  iso bank : $ROOT/isos"
echo "  extracted: $ROOT/extracted"
echo "  URL      : http://127.0.0.1:$PORT"
echo "  manifest : http://127.0.0.1:$PORT/api/lan/sync/manifest?preset=active"
echo

# Requires extract-xiso on PATH (the seed builds + extracts a placeholder XISO).
command -v extract-xiso >/dev/null || {
  echo "WARNING: extract-xiso not found on PATH — the ISO seed step will fail." >&2
}

exec env \
  SEED_LAN_SYNC=true \
  CONTAINERS_ISO_DIR="$ROOT/isos" \
  LAN_SYNC_EXTRACT_DIR="$ROOT/extracted" \
  go run -tags dev ./cmd/server serve --dir "$ROOT/pb_data" --http "127.0.0.1:$PORT"
