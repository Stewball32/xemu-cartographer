#!/usr/bin/env bash
#
# deploy-beta.sh — deploy the BETA (test) tier.
#
# The gate before prod: same build, fully isolated run dir / port / pb_data /
# container namespace. Prove migrations and behaviour here before prod.
#
# Cartographer runs NATIVELY (not Docker) — it provisions xemu containers and
# needs host podman + /dev/kvm + /dev/dri — so a "deploy" is: build the frontend
# and the server binary from the working tree, copy that snapshot into the tier's
# run dir, restart it, and health-check. The INTERFACE matches the
# stew-site-template standard (deploy-pre.sh / deploy-prod.sh).
#
# Uniform tier interface:
#   ./deploy-beta.sh          guard → build → deploy → health-check
#   ./deploy-beta.sh down     stop the tier (keeps pb_data)
#   ./deploy-beta.sh logs     follow logs
#
# Guards (override only when you mean it):
#   ALLOW_ANY_BRANCH=1   deploy from a branch other than BETA_BRANCH
#   ALLOW_DIRTY=1        deploy with uncommitted changes
#   SKIP_BACKUP=1        don't snapshot pb_data first
#
# Ports come from this project's block in PORTS.md (beta/test = 18099).
set -euo pipefail
cd "$(dirname "$0")"
# shellcheck source=scripts/deploy-lib.sh
. scripts/deploy-lib.sh

TIER="beta (test)"
BETA_BRANCH="${BETA_BRANCH:-beta}"
BETA_DIR="${BETA_DIR:-$HOME/xcarto-beta}"
RUNNER="run-beta.sh"

# Port: from the tier's .env (single source), falling back to the PORTS.md block.
PORT="$(grep -E '^PUBLIC_PB_PORT=' "$BETA_DIR/.env" 2>/dev/null | cut -d= -f2- | tr -d '"' || true)"
PORT="${PORT:-18099}"

case "${1:-up}" in
  down)
    tier_stop "$PORT" "$TIER"
    exit 0
    ;;
  logs)
    exec tail -f "$BETA_DIR/beta.log"
    ;;
esac

require_cmd git curl go pnpm ss
require_dir "$BETA_DIR" "the beta run dir is missing — see docs/DEPLOYMENTS.md"
require_file "$BETA_DIR/.env" "copy .env.example to $BETA_DIR/.env and fill it in"
require_file "$BETA_DIR/$RUNNER" "the beta run script is missing"

say "── ${TIER} ───────────────────────────────────────────────"
info "branch expected : ${BETA_BRANCH}"
info "run dir         : ${BETA_DIR}"
info "host port       : 127.0.0.1:${PORT}"
info "public          : https://beta.norcal.pro (cloudflared)"
say

# A deploy builds from the working tree — so pin what's being shipped.
require_branch "$BETA_BRANCH"
require_clean_tree

[ "${SKIP_BACKUP:-0}" = "1" ] || backup_pb_data "$BETA_DIR"

tier_stop "$PORT" "$TIER"
build_snapshot "$BETA_DIR" "$PORT"
tier_start "$BETA_DIR" "$RUNNER" "$TIER"
wait_healthy "$TIER" "$PORT" "$BETA_DIR/beta.log"

say
ok "beta deployed. Verify:"
info "./deploy-beta.sh logs                     # migration errors? bot connected?"
info "curl -sI http://127.0.0.1:${PORT}/"
info "https://beta.norcal.pro"
