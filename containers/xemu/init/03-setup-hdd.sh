#!/bin/bash
# =============================================================================
# 03-setup-hdd.sh
# Ensures an HDD image is set in the toml under [sys.files].
# Checks for an instance-specific HDD in /shared/hdds/$HOSTNAME.qcow2.
# If not found, copies the default HDD and updates the toml.
# =============================================================================

ENV_FILE="/custom-cont-init.d/.env"
if [ -f "$ENV_FILE" ]; then
    source "$ENV_FILE"
fi

if [ -z "$TOML_PATH" ] || [ -z "$HDD_DIR" ]; then
    echo "[03-setup-hdd] ERROR: TOML_PATH and HDD_DIR must be set."
    exit 1
fi

CURRENT_TOML="$TOML_PATH/xemu.toml"
DEFAULT_HDD="$HDD_DIR/${DEFAULT_HDD_NAME:-default.qcow2}"
HDD_EXT="${DEFAULT_HDD##*.}"
INSTANCE_HDD="$HDD_DIR/$HOSTNAME.$HDD_EXT"

# -----------------------------------------------------------------------------
# Check if hdd_path is already set in the toml
# -----------------------------------------------------------------------------
if grep -q "^hdd_path\s*=" "$CURRENT_TOML"; then
    echo "[03-setup-hdd] hdd_path already set in toml, skipping."
    exit 0
fi

# -----------------------------------------------------------------------------
# Check for instance-specific HDD
# -----------------------------------------------------------------------------
if [ ! -f "$INSTANCE_HDD" ]; then
    echo "[03-setup-hdd] No instance HDD found at $INSTANCE_HDD, copying default."
    if [ ! -f "$DEFAULT_HDD" ]; then
        echo "[03-setup-hdd] ERROR: Default HDD not found at $DEFAULT_HDD."
        exit 1
    fi
    cp "$DEFAULT_HDD" "$INSTANCE_HDD"
    # Init runs as root inside the container, so the copy is root-owned.
    # Hand it back to the PUID/PGID the actual xemu process runs as so
    # subsequent reads from xemu (and host-side cleanup) don't need sudo.
    chown "${PUID:-1000}:${PGID:-1000}" "$INSTANCE_HDD"
    echo "[03-setup-hdd] Copied $DEFAULT_HDD -> $INSTANCE_HDD (owner=${PUID:-1000}:${PGID:-1000})"
else
    echo "[03-setup-hdd] Found instance HDD at $INSTANCE_HDD."
fi

# -----------------------------------------------------------------------------
# Update toml with HDD path
# -----------------------------------------------------------------------------
if grep -q "^\[sys\.files\]" "$CURRENT_TOML"; then
    sed -i "/^\[sys\.files\]/a hdd_path = '$INSTANCE_HDD'" "$CURRENT_TOML"
    echo "[03-setup-hdd] Injected hdd_path under existing [sys.files]."
else
    printf "\n[sys.files]\nhdd_path = '$INSTANCE_HDD'\n" >> "$CURRENT_TOML"
    echo "[03-setup-hdd] Appended [sys.files] block with hdd_path = '$INSTANCE_HDD'."
fi

echo "[03-setup-hdd] Done."
