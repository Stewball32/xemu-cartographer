#!/bin/bash
# =============================================================================
# 02-patch-toml.sh
# Idempotent patches for xemu.toml — runs on every container startup.
# Ensures netif is set under [net.pcap] and qmp_socket_path under [machine].
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
# with backend='pcap' and netif is empty. Read /sys/class/net directly (don't
# depend on `ip`), skipping loopback + virtual devices to land on the physical NIC
# (container shares host netns via --network host).
NETIF=""
for cand in /sys/class/net/*; do
    [ -e "$cand" ] || continue
    name=$(basename "$cand")
    case "$name" in
        lo | docker* | cni* | veth* | br-* | virbr* | podman* | tap* | tun* | kube*) continue ;;
    esac
    NETIF="$name"
    break
done
if [ -z "$NETIF" ]; then
    for cand in /sys/class/net/*; do
        [ -e "$cand" ] || continue
        name=$(basename "$cand")
        [ "$name" = "lo" ] && continue
        NETIF="$name"
        break
    done
fi

if [ -z "$NETIF" ]; then
    echo "[02-patch-toml] WARNING: No network interface found, skipping netif patch."
elif grep -q "^netif\s*=" "$CURRENT_TOML"; then
    echo "[02-patch-toml] Network interface already set, skipping."
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

echo "[02-patch-toml] Done."
