#!/bin/bash
# =============================================================================
# 01-setup-toml.sh
# Copies xemu.toml from /shared/toml if it doesn't exist in /config.
# If the toml already exists, exits early and leaves it untouched.
# Patching is handled by 02-patch-toml.sh (runs on every startup).
# =============================================================================

ENV_FILE="/custom-cont-init.d/.env"
if [ -f "$ENV_FILE" ]; then
    source "$ENV_FILE"
fi

if [ -z "$DEFAULT_TOML" ] || [ -z "$TOML_PATH" ]; then
    echo "[01-setup-toml] ERROR: DEFAULT_TOML and TOML_PATH must be set."
    exit 1
fi

CURRENT_TOML="$TOML_PATH/xemu.toml"

# -----------------------------------------------------------------------------
# Check if toml already exists — if so, nothing to do
# -----------------------------------------------------------------------------
if [ -f "$CURRENT_TOML" ]; then
    echo "[01-setup-toml] $CURRENT_TOML already exists, skipping."
    exit 0
fi

# -----------------------------------------------------------------------------
# Copy default toml
# -----------------------------------------------------------------------------
if [ ! -f "$DEFAULT_TOML" ]; then
    echo "[01-setup-toml] ERROR: $DEFAULT_TOML not found."
    exit 1
fi

mkdir -p "$TOML_PATH"
cp "$DEFAULT_TOML" "$CURRENT_TOML"
echo "[01-setup-toml] Copied $DEFAULT_TOML -> $CURRENT_TOML"
echo "[01-setup-toml] Done."
