#!/usr/bin/env bash
#
# migrate-squash.sh — collapse this branch's granular migrations into ONE
# release migration, on the way to main.
#
#   dev/test : many small migrations (Automigrate + `migrate create`) — messy is fine
#   main     : only squashed release migrations — one file per approved version
#
# What it does:
#   1. classifies migrations/ into RELEASE-line and GRANULAR files
#   2. applies ALL of them to a throwaway database → the current end state
#   3. snapshots that state into migrations/<ts>_release_<version>.go
#   4. moves the granular files into .migrate-archive/<version>/ (gitignored)
#      so ./scripts/migrate-verify.sh can prove the squash is equivalent
#
# Usage:
#   ./scripts/migrate-squash.sh --version v0.3.0 [--from <pb_data-dir>]
#                               [--delete-missing] [--keep-granular]
#
#   --version         release label for the migration filename (required)
#   --from            start from a COPY of this pb_data instead of an empty DB
#                     (use a copy of prod to squash exactly prod's upgrade path)
#   --delete-missing  make the release migration authoritative: it will DELETE
#                     collections not present in the snapshot. Needed only when
#                     this release removes a collection — migrate-verify.sh tells
#                     you when. Destructive; off by default.
#   --keep-granular   leave the granular files in migrations/ (inspection only;
#                     do NOT merge to main in this state)
#
# Run ./scripts/migrate-verify.sh afterwards — always — before merging.
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION=""; FROM=""; DELETE_MISSING=0; KEEP_GRANULAR=0
die() { printf '✗ %s\n' "$*" >&2; exit 1; }
say() { printf '%s\n' "$*"; }

while [ $# -gt 0 ]; do
  case "$1" in
    --version)        VERSION="${2:-}"; shift 2 ;;
    --from)           FROM="${2:-}"; shift 2 ;;
    --delete-missing) DELETE_MISSING=1; shift ;;
    --keep-granular)  KEEP_GRANULAR=1; shift ;;
    -h|--help)        sed -n '2,32p' "$0"; exit 0 ;;
    *)                die "unknown argument: $1" ;;
  esac
done
[ -n "$VERSION" ] || die "--version is required (e.g. --version v0.3.0)"

# shellcheck source=scripts/migrate-lib.sh
. scripts/migrate-lib.sh

mapfile -t GRANULAR < <(list_granular)
mapfile -t RELEASES < <(list_releases)

say "── squash ────────────────────────────────────────────────"
say "  version        : $VERSION"
say "  release-line   : ${#RELEASES[@]} file(s)"
say "  granular       : ${#GRANULAR[@]} file(s)"
[ -n "$FROM" ] && say "  starting data  : $FROM (copied)"
say "──────────────────────────────────────────────────────────"

[ "${#GRANULAR[@]}" -gt 0 ] || die "no granular migrations to squash — migrations/ already holds only release files"

for f in "${GRANULAR[@]}"; do say "    squashing: $(basename "$f")"; done
say

# 1. Apply every current migration to a throwaway DB → the end state.
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
DATA="$WORK/pb_data"
seed_data_dir "$FROM" "$DATA"

BIN="$WORK/server"
say "building current tree…"
go build -o "$BIN" ./cmd/server || die "go build failed"

say "applying all migrations (release + granular) to a throwaway DB…"
apply_migrations "$BIN" "$DATA" || die "migrations failed to apply — fix them before squashing"

# 2. Snapshot that end state into migrations/.
say "snapshotting the resulting schema…"
before="$(ls migrations/*.go 2>/dev/null | wc -l)"
yes | "$BIN" migrate collections --dir "$DATA" >/dev/null 2>&1 || true
after="$(ls migrations/*.go 2>/dev/null | wc -l)"
[ "$after" -gt "$before" ] || die "no snapshot was generated (schema unchanged?)"

SNAP="$(ls -t migrations/*_collections_snapshot.go | head -1)"
TS="$(basename "$SNAP" | cut -d_ -f1)"
RELEASE="migrations/${TS}_release_${VERSION//[^A-Za-z0-9]/_}.go"
mv "$SNAP" "$RELEASE"

# Snapshots import with deleteMissing=false: they create/update collections but
# never remove one. If this release DELETES a collection, the squash must say so.
if [ "$DELETE_MISSING" = "1" ]; then
  sed -i 's/ImportCollectionsByMarshaledJSON(\[\]byte(jsonData), false)/ImportCollectionsByMarshaledJSON([]byte(jsonData), true)/' "$RELEASE"
  say "! release migration set to deleteMissing=TRUE (removes collections absent from the snapshot)"
fi

say "  → $RELEASE"

# 3. Archive the granular files so verify can rebuild the "before" side.
ARCHIVE=".migrate-archive/${VERSION}"
mkdir -p "$ARCHIVE"
for f in "${GRANULAR[@]}"; do cp "$f" "$ARCHIVE/"; done
say "  archived ${#GRANULAR[@]} granular file(s) → $ARCHIVE/"

if [ "$KEEP_GRANULAR" = "1" ]; then
  say "! --keep-granular: granular files left in migrations/ — do NOT merge to main like this"
else
  for f in "${GRANULAR[@]}"; do rm -f "$f"; done
  say "  removed granular files from migrations/"
fi

say
say "✓ squashed into $(basename "$RELEASE")"
say
say "NEXT — do not merge until this passes:"
say "  ./scripts/migrate-verify.sh --from <copy-of-prod-pb_data>"
say "  # then: git add -A && git commit -m 'chore(release): squash migrations for ${VERSION}'"
