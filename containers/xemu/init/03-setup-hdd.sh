#!/bin/bash
# =============================================================================
# 03-setup-hdd.sh
# Ensures hdd_path under [sys.files] points at this instance's qcow2 OVERLAY.
#
# The overlay is a thin copy-on-write layer over the shared, read-only ROOT
# image ($HDD_DIR/$DEFAULT_HDD_NAME). It records only this instance's deltas;
# xemu reads the canonical disk from the root through qcow2's backing chain.
#
# The overlay is normally PRE-CREATED on the host by the podman provisioner
# (internal/podman, Manager.Create) before the container starts — this script
# then just injects its path into the toml. If the overlay is missing (e.g. a
# standalone container run outside the provisioner) it is created here with
# qemu-img when available. We NEVER fall back to a full copy of the multi-GiB
# root — that slow, disk-heavy copy is exactly what the overlay scheme replaces.
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
ROOT_HDD_NAME="${DEFAULT_HDD_NAME:-_default.qcow2}"
ROOT_HDD="$HDD_DIR/$ROOT_HDD_NAME"
# Overlays are always qcow2 regardless of the root's extension.
INSTANCE_HDD="$HDD_DIR/$HOSTNAME.qcow2"

# -----------------------------------------------------------------------------
# Check if hdd_path is already set in the toml
# -----------------------------------------------------------------------------
if grep -q "^hdd_path\s*=" "$CURRENT_TOML"; then
    echo "[03-setup-hdd] hdd_path already set in toml, skipping."
    exit 0
fi

# -----------------------------------------------------------------------------
# Ensure the instance overlay exists (host provisioner normally created it)
# -----------------------------------------------------------------------------
if [ ! -f "$INSTANCE_HDD" ]; then
    echo "[03-setup-hdd] Overlay $INSTANCE_HDD missing; attempting in-container creation."
    if [ ! -f "$ROOT_HDD" ]; then
        echo "[03-setup-hdd] ERROR: root image $ROOT_HDD not found; cannot create overlay."
        exit 1
    fi
    if ! command -v qemu-img >/dev/null 2>&1; then
        echo "[03-setup-hdd] ERROR: overlay missing and qemu-img unavailable in-container."
        echo "[03-setup-hdd]        The host provisioner (internal/podman) should create"
        echo "[03-setup-hdd]        $INSTANCE_HDD before start. Aborting (no full-copy fallback)."
        exit 1
    fi
    # Relative backing: the root and overlay share $HDD_DIR, so the bare basename
    # resolves regardless of the mount's absolute path. Create from $HDD_DIR so
    # the stored backing string is the relative name, not an absolute path.
    if ( cd "$HDD_DIR" && qemu-img create -f qcow2 -b "$ROOT_HDD_NAME" -F qcow2 "$HOSTNAME.qcow2" ); then
        # Init runs as root; hand the overlay to the uid xemu actually runs as so
        # xemu can write its deltas and host-side cleanup doesn't need sudo.
        chown "${PUID:-1000}:${PGID:-1000}" "$INSTANCE_HDD" 2>/dev/null || true
        echo "[03-setup-hdd] Created overlay $INSTANCE_HDD (backing $ROOT_HDD_NAME, owner=${PUID:-1000}:${PGID:-1000})."
    else
        echo "[03-setup-hdd] ERROR: qemu-img create overlay failed."
        exit 1
    fi
else
    echo "[03-setup-hdd] Found overlay $INSTANCE_HDD."
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
