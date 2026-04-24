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
NETIF=$(ip -o link show | awk -F': ' '{print $2}' | grep -v lo | head -n1)

if [ -z "$NETIF" ]; then
    echo "[02-patch-toml] WARNING: No network interface found, skipping netif patch."
elif grep -q "^netif\s*=" "$CURRENT_TOML"; then
    echo "[02-patch-toml] Network interface already set, skipping."
elif grep -q "^\[net\.pcap\]" "$CURRENT_TOML"; then
    sed -i "/^\[net\.pcap\]/a netif = '$NETIF'" "$CURRENT_TOML"
    echo "[02-patch-toml] Injected netif = '$NETIF' under existing [net.pcap]."
else
    printf "\n[net]\nenable = true\nbackend = 'pcap'\n\n[net.pcap]\nnetif = '$NETIF'\n" >> "$CURRENT_TOML"
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
