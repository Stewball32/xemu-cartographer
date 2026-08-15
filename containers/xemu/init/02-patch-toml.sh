#!/bin/bash
# =============================================================================
# 02-patch-toml.sh
# Idempotent patches for xemu.toml — runs on every container startup.
# Ensures netif under [net.pcap], qmp_socket_path under [machine], and dvd_path
# under [sys.files] when a game ISO is bind-mounted at /game.iso.
# Each patch checks if the value is already present and skips if so.
# =============================================================================

ENV_FILE="/custom-cont-init.d/.env"
if [ -f "$ENV_FILE" ]; then
    source "$ENV_FILE"
fi

if [ -z "$TOML_PATH" ]; then
    echo "[02-patch-toml] ERROR: TOML_PATH must be set."
    exit 1
fi

CURRENT_TOML="$TOML_PATH/xemu.toml"

if [ ! -f "$CURRENT_TOML" ]; then
    echo "[02-patch-toml] ERROR: $CURRENT_TOML not found (01-setup-toml should have created it)."
    exit 1
fi

# -----------------------------------------------------------------------------
# Patch: Network interface
# Ensures netif is set under [net.pcap], appending the full block if needed.
# -----------------------------------------------------------------------------
# Resolve a real interface for the pcap backend. xemu aborts when [net] is enabled
# with backend='pcap' and netif is empty, and binds the WRONG network (breaking
# System Link with real Xboxes) when it's set to an interface that isn't on the
# console subnet. Read /sys/class/net directly (don't depend on `ip`).
#
# Precedence: an explicit NETIF from the sourced .env wins outright. It's the only
# way to be correct on a host with several plausible physical NICs — autodetect
# cannot know which one the Xboxes are on. Otherwise autodetect below.
#
# NETIF may already be set in the environment by $ENV_FILE (sourced at the top of
# this script), so capture it BEFORE the detection loop overwrites the variable.
NETIF_OVERRIDE="${NETIF:-}"
NETIF=""

# netif_class: classify a candidate as skip / fallback / preferred.
#
# Interfaces are iterated in glob (i.e. alphabetical) order, so anything sorting
# before the real NIC wins by default — `bond0`, `br0`, and `eno1` all sort
# before `enp191s0`. The old list only matched `br-*` (docker's bridge naming)
# and missed plain `br0` entirely.
#
# skip — never valid for pcap System Link:
#   lo/docker/cni/veth/br/virbr/podman/tap/tun/kube — virtual or container plumbing
#   bond/dummy/ifb/sit/gre/ip6tnl                   — aggregates + tunnels
#   wl*/ww*                                         — wireless (wlan0, wlp*, wwan*)
#   tailscale*/wg*/zt*/nebula*/utun*                — overlay VPNs
#
# fallback — `eno*` is a REAL onboard NIC, not virtual plumbing, so excluding it
# outright would strand any host whose ONLY interface is eno1 (the common case on
# server hardware). It is merely DEPRIORITISED, so on a multi-NIC box it can no
# longer beat enp*/ens* by alphabetical accident. Which physical NIC is on the
# Xbox subnet is genuinely unknowable here — that ambiguity is what NETIF is for.
netif_class() {
    case "$1" in
        lo | docker* | cni* | veth* | br-* | br[0-9]* | virbr* | podman* | tap* | tun* | kube*) echo skip ;;
        bond* | dummy* | ifb* | sit* | gre* | ip6tnl*) echo skip ;;
        wl* | ww*) echo skip ;;
        tailscale* | wg* | zt* | nebula* | utun*) echo skip ;;
        eno*) echo fallback ;;
        *) echo preferred ;;
    esac
}

# has_carrier: the NIC must have a link partner. A cabled-but-unplugged port, or
# a wired NIC on a host whose real uplink is wireless, reports carrier 0 (or
# ENODEV on read) and is useless for System Link. This is what keeps autodetect
# off a NO-CARRIER interface that merely sorts first.
has_carrier() {
    [ "$(cat "/sys/class/net/$1/carrier" 2>/dev/null)" = "1" ]
}

if [ -n "$NETIF_OVERRIDE" ]; then
    NETIF="$NETIF_OVERRIDE"
    # Warn but obey: the operator set this deliberately, and a NIC can come up
    # after this script runs (xemu opens the pcap handle later).
    if [ ! -e "/sys/class/net/$NETIF" ]; then
        echo "[02-patch-toml] WARNING: NETIF override '$NETIF' does not exist on this host; using it anyway."
    elif ! has_carrier "$NETIF"; then
        echo "[02-patch-toml] WARNING: NETIF override '$NETIF' has no carrier; using it anyway."
    fi
    echo "[02-patch-toml] Using NETIF override '$NETIF' from $ENV_FILE."
else
    # Two passes: take a preferred physical NIC if one has carrier, otherwise
    # accept a deprioritised onboard eno*.
    for tier in preferred fallback; do
        [ -n "$NETIF" ] && break
        for cand in /sys/class/net/*; do
            [ -e "$cand" ] || continue
            name=$(basename "$cand")
            [ "$(netif_class "$name")" = "$tier" ] || continue
            has_carrier "$name" || continue
            NETIF="$name"
            break
        done
    done
    if [ -n "$NETIF" ]; then
        echo "[02-patch-toml] Autodetected carrier-up interface '$NETIF'."
    else
        # Nothing qualified. Rather than silently binding pcap to a virtual
        # interface (the old behaviour — it took ANY non-lo interface, which on
        # this host class means a podman bridge and a dead System Link), leave
        # netif alone and say so. A wrong netif looks like "Halo is laggy"; an
        # absent one is a clear xemu-side error.
        echo "[02-patch-toml] WARNING: No carrier-up physical interface found." >&2
        echo "[02-patch-toml]          Set NETIF=<iface> in $ENV_FILE to override." >&2
    fi
fi

# Compare-and-rewrite, NOT skip-if-present. The old code skipped whenever any
# netif line existed, so a value baked in on a different machine (or from a NIC
# that has since been renamed/removed) survived forever and silently bound xemu
# to the wrong network. The toml is a persistent per-instance bind mount, so
# that state is real and long-lived.
CURRENT_NETIF=$(sed -n "s/^netif[[:space:]]*=[[:space:]]*'\([^']*\)'.*/\1/p" "$CURRENT_TOML" | head -n1)

if [ -z "$NETIF" ]; then
    echo "[02-patch-toml] Skipping netif patch (nothing resolved; leaving existing value '$CURRENT_NETIF')."
elif [ -n "$CURRENT_NETIF" ] && [ "$CURRENT_NETIF" = "$NETIF" ]; then
    echo "[02-patch-toml] Network interface already '$NETIF', no change."
elif [ -n "$CURRENT_NETIF" ]; then
    # Rewrite in place; keeps the line where it is so the surrounding table
    # structure is untouched.
    sed -i "s/^netif[[:space:]]*=.*/netif = '$NETIF'/" "$CURRENT_TOML"
    echo "[02-patch-toml] Corrected stale netif '$CURRENT_NETIF' -> '$NETIF'."
elif grep -q "^\[net\.pcap\]" "$CURRENT_TOML"; then
    sed -i "/^\[net\.pcap\]/a netif = '$NETIF'" "$CURRENT_TOML"
    echo "[02-patch-toml] Injected netif = '$NETIF' under existing [net.pcap]."
elif grep -q "^\[net\]" "$CURRENT_TOML"; then
    # [net] exists but xemu dropped the empty [net.pcap]; append only the subtable.
    # A second [net] would be a duplicate table (invalid TOML) that xemu refuses.
    printf "\n[net.pcap]\nnetif = '%s'\n" "$NETIF" >> "$CURRENT_TOML"
    echo "[02-patch-toml] Appended [net.pcap] with netif = '$NETIF' under existing [net]."
else
    printf "\n[net]\nenable = true\nbackend = 'pcap'\n\n[net.pcap]\nnetif = '%s'\n" "$NETIF" >> "$CURRENT_TOML"
    echo "[02-patch-toml] Appended full [net] block with netif = '$NETIF'."
fi

# -----------------------------------------------------------------------------
# Patch: QMP socket path
# Ensures qmp_socket_path is set under [machine], using $HOSTNAME for the
# socket name so each instance gets a unique socket.
# -----------------------------------------------------------------------------
QMP_SOCK="/qmp/$HOSTNAME.sock"

if grep -q "^qmp_socket_path\s*=" "$CURRENT_TOML"; then
    echo "[02-patch-toml] QMP socket path already set, skipping."
elif grep -q "^\[machine\]" "$CURRENT_TOML"; then
    sed -i "/^\[machine\]/a qmp_socket_path = '$QMP_SOCK'" "$CURRENT_TOML"
    echo "[02-patch-toml] Injected qmp_socket_path = '$QMP_SOCK' under existing [machine]."
else
    printf "\n[machine]\nqmp_socket_path = '$QMP_SOCK'\n" >> "$CURRENT_TOML"
    echo "[02-patch-toml] Appended [machine] block with qmp_socket_path = '$QMP_SOCK'."
fi

# -----------------------------------------------------------------------------
# Patch: DVD / game disc
# When the host provisioner bind-mounts a per-instance game ISO at $DVD_MOUNT
# (internal/podman containerDVDPath, set from CreateOptions.GameISO or
# Config.DVDPath), point xemu's dvd_path at it so the disc is ATTACHED at boot.
# This is THE wire that makes "attach ISO -> boots straight into the game" work:
# a game disc present at cold boot is auto-launched by the master image's
# Cerbios/Xbox boot path (verified live 2026-07-10 — Halo 2.iso boots Halo 2,
# Halo CE.iso boots Halo, no disc boots the UnleashX dashboard; see ADR-0004).
# No master-image config change is needed for that; the optional
# cmd/master-autolaunch lever only forces UnleashX's <DVD AutoLaunch>. No ISO
# mounted (file absent) -> HDD-only boot, no dvd_path. Idempotent.
# -----------------------------------------------------------------------------
DVD_MOUNT="/game.iso"

if grep -q "^dvd_path\s*=" "$CURRENT_TOML"; then
    echo "[02-patch-toml] DVD path already set, skipping."
elif [ ! -f "$DVD_MOUNT" ]; then
    echo "[02-patch-toml] No DVD mounted at $DVD_MOUNT, skipping dvd_path (HDD-only boot)."
elif grep -q "^\[sys\.files\]" "$CURRENT_TOML"; then
    sed -i "/^\[sys\.files\]/a dvd_path = '$DVD_MOUNT'" "$CURRENT_TOML"
    echo "[02-patch-toml] Injected dvd_path = '$DVD_MOUNT' under existing [sys.files]."
else
    printf "\n[sys.files]\ndvd_path = '%s'\n" "$DVD_MOUNT" >> "$CURRENT_TOML"
    echo "[02-patch-toml] Appended [sys.files] block with dvd_path = '$DVD_MOUNT'."
fi

echo "[02-patch-toml] Done."
