#!/bin/bash
# =============================================================================
# 04-patch-startwm.sh
# Makes the labwc compositor resilient to a GPU it cannot render on — runs on
# every container startup (idempotent).
#
# wlroots (labwc's backend) HARD-ABORTS when it cannot initialize an EGL or
# Vulkan renderer on the DRM node it is handed — e.g. a driverless NVIDIA render
# node in device passthrough ("DRI2: failed to create screen" / "unable to
# create renderer"). Unlike xemu/selkies (which fall back to llvmpipe), labwc
# just exits, and svc-de's startwm loop relaunches it ~1/sec → permanent black
# screen.
#
# Rather than force software compositing everywhere (which would needlessly slow
# hosts whose GPU works in-container), this patch wraps the bare `labwc` calls in
# /defaults/startwm_wayland.sh with a shell function that TRIES HARDWARE FIRST and
# falls back to the software (pixman) renderer only if labwc dies on startup.
# Hosts with a working GPU keep hardware compositing; broken-GPU hosts self-heal.
# =============================================================================

STARTWM="/defaults/startwm_wayland.sh"

if [ ! -f "$STARTWM" ]; then
    echo "[04-patch-startwm] WARNING: $STARTWM not found, skipping."
    exit 0
fi

# Drop any earlier hard-coded "export WLR_RENDERER=pixman" stopgap so the wrapper
# below is free to try hardware first (no-op on a pristine image).
sed -i '/^export WLR_RENDERER=pixman[[:space:]]*$/d' "$STARTWM"

if grep -q "labwc-renderer-fallback" "$STARTWM"; then
    echo "[04-patch-startwm] Fallback wrapper already present, skipping."
    exit 0
fi

# Build the wrapper verbatim (single-quoted heredoc = no expansion at patch time).
# A bash function named `labwc` shadows the bare `labwc` calls in startwm; the
# inner `command labwc` reaches the real binary without recursing. A renderer
# init failure is immediate, so a sub-3s exit means "GPU renderer unavailable" —
# relaunch forcing pixman. The `-z "$WLR_RENDERER"` guard avoids a second retry.
WRAPPER_FILE="$(mktemp)"
cat > "$WRAPPER_FILE" <<'WRAPPER'

# --- labwc-renderer-fallback (injected by 04-patch-startwm.sh) ---------------
labwc() {
    local _start=$SECONDS
    command labwc "$@"
    local _rc=$?
    if [ $((SECONDS - _start)) -lt 3 ] && [ -z "$WLR_RENDERER" ]; then
        echo "[startwm] labwc exited in <3s (rc=$_rc); GPU renderer unavailable, retrying with software (pixman)" \
            | tee -a /config/.labwc-renderer.log >&2
        WLR_RENDERER=pixman command labwc "$@"
        _rc=$?
    fi
    return $_rc
}
# -----------------------------------------------------------------------------
WRAPPER

# Insert the wrapper right after the shebang (line 1), before the labwc calls.
sed -i "1r $WRAPPER_FILE" "$STARTWM"
rm -f "$WRAPPER_FILE"
chmod +x "$STARTWM"

echo "[04-patch-startwm] Installed labwc hardware→pixman renderer fallback in $STARTWM."
echo "[04-patch-startwm] Done."
