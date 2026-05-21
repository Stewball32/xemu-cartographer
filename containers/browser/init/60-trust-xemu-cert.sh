#!/bin/sh
# Make the xemu container's self-signed CA trusted by Firefox so the kiosk
# loads https://localhost:<XemuHTTPS> without a warning. The CA is generated
# by internal/podman/cert.go:generateXemuCerts and bind-mounted read-only at
# /xemu-cert.
#
# Two independent mechanisms run in parallel so at least one works regardless
# of which Firefox build/profile-layout jlesage's image happens to ship:
#
#   1. Mozilla "policies.json" — system-wide enterprise policy that installs
#      the CA into Firefox's trust store. Works on any modern Firefox, no
#      extra packages needed, ignores per-profile NSS DB location.
#   2. certutil into the per-profile NSS DB at /config/profile/cert9.db —
#      legacy path, requires nss-tools. Kept as a belt-and-suspenders fallback
#      because some Firefox builds ignore policies.json silently when run
#      without the right install prefix.
set -eu

CERT=/xemu-cert/ca.pem

if [ ! -f "$CERT" ]; then
    echo "[trust-xemu-cert] $CERT not found, skipping"
    exit 0
fi

# ---------------------------------------------------------------------------
# Mechanism 1: Mozilla policies.json
# ---------------------------------------------------------------------------
# Drop a policies.json into every well-known Firefox policy dir. Firefox
# checks all of these on startup; an extra file in a non-existent install
# path is harmless. Inline JSON keeps the script self-contained.
POLICY_JSON='{"policies":{"Certificates":{"ImportEnterpriseRoots":true,"Install":["/xemu-cert/ca.pem"]}}}'

for dir in \
    /etc/firefox/policies \
    /usr/lib/firefox/distribution \
    /usr/lib/firefox-esr/distribution \
    /opt/firefox/distribution
do
    if mkdir -p "$dir" 2>/dev/null; then
        if printf '%s\n' "$POLICY_JSON" > "$dir/policies.json"; then
            echo "[trust-xemu-cert] wrote $dir/policies.json"
        fi
    fi
done

# ---------------------------------------------------------------------------
# Mechanism 2: certutil into the per-profile NSS DB
# ---------------------------------------------------------------------------
PROFILE=/config/profile

if ! command -v certutil >/dev/null 2>&1; then
    echo "[trust-xemu-cert] certutil missing, attempting apk add nss-tools"
    if ! apk add --no-cache nss-tools >/dev/null 2>&1; then
        echo "[trust-xemu-cert] apk add nss-tools failed; relying on policies.json only"
        exit 0
    fi
fi

mkdir -p "$PROFILE"
if [ ! -f "$PROFILE/cert9.db" ]; then
    certutil -N -d "sql:$PROFILE" --empty-password
    echo "[trust-xemu-cert] initialised empty NSS DB at $PROFILE"
fi

# -A re-imports cleanly if the nickname already exists, so this stays
# idempotent across container restarts.
if certutil -A -n "xemu-cartographer dev CA" -t "C,," -i "$CERT" -d "sql:$PROFILE"; then
    echo "[trust-xemu-cert] imported $CERT into $PROFILE via certutil"
else
    echo "[trust-xemu-cert] certutil import failed; relying on policies.json only"
fi
