#!/usr/bin/env bash
# launch-h2-instance.sh N  — bring up ONE of the networked Halo 2 SYSTEM-LINK instances.
# H2 counterpart of launch-instance.sh (CE): same hardened-pad + per-instance TOML +
# sigshield survival + ptrace shim, but the H2 ISO, per-instance H2 overlay, a distinct
# MAC eeprom, and a [net] udp backend wired THROUGH the reflector so two instances share
# one L2 segment and discover each other on Halo 2 System Link.
#
# Pair with:  udp_reflector.py --bind 9975,9976 --refl 9985,9986   (the H2 hub)
# Distinct from the CE 3way rig (ports 997x/998x, eeprom1/2, names ce-3way-N).
set -uo pipefail
i="${1:?usage: launch-h2-instance.sh N (1|2)}"
D=/home/stew/.local/share/xemu/xemu
T=$D/3way
HARNESS_PAD=/home/stew/repos/xemu-cartographer-harness/scripts/runtime/padpool.py
ISO='/home/stew/repos/halo-offset-mapper/iso/Halo 2.iso'
SHIM=$D/cart-ptrace-shim.so

# ---- per-instance parameters (distinct MAC eeprom + own overlay + own net ports) ----
case "$i" in
  1) PADBASE=0x0510; EEPROM=$D/eeprom3.bin; HDD=$T/ovl-h2-1.qcow2; BINDP=9975; REMP=9985;;
  2) PADBASE=0x0520; EEPROM=$D/eeprom4.bin; HDD=$T/ovl-h2-2.qcow2; BINDP=9976; REMP=9986;;
  *) echo "instance must be 1|2"; exit 2;;
esac
NAME=h2sl-$i
RD=/run/user/$(id -u)/xemu-harness/3way-h2sl-$i
QMP=/run/user/$(id -u)/xemu-qmp-3way-h2sl-$i.sock
TOML=$T/xemu-3way-h2sl-$i.toml
PIDFILE=$T/logs/xemu-h2sl-$i.pid
XLOG=$T/logs/xemu-h2sl-$i.out

# ---- 1. hardened pad (port1) — one player per networked console -----------------
pkill -KILL -f "[p]adpool.py --count .* --rundir $RD" 2>/dev/null || true
rm -rf "$RD"; mkdir -p "$RD" "$T/logs"
setsid python3 "$HARNESS_PAD" --count 1 --rundir "$RD" --base-version "$PADBASE" \
       >"$RD/padpool.out" 2>&1 </dev/null &
for _ in $(seq 1 50); do [ -f "$RD/pads.json" ] && break; sleep 0.2; done
[ -f "$RD/pads.json" ] || { echo "instance $i: padpool failed"; cat "$RD/padpool.out"; exit 1; }
GUID=$(python3 -c "import json;print([p['guid'] for p in json.load(open('$RD/pads.json'))['pads'] if p['port']==1][0])")
echo "h2sl-$i: pad guid=$GUID"

# ---- 2. build the TOML ---------------------------------------------------------
cat > "$TOML" <<TOML
[general]
show_welcome = false

[display.window]
startup_size = '640x480'
vsync = false

[display.ui]
fit = 'center'
auto_scale = false

[input]
auto_bind = false
background_input_capture = true
gamepad_mappings = [
  { gamepad_id = '$GUID' },
]

[input.bindings]
port1_driver = 'usb-xbox-gamepad'
port1 = '$GUID'

[net]
enable = true
backend = 'udp'

[net.udp]
bind_addr = '0.0.0.0:$BINDP'
remote_addr = '127.0.0.1:$REMP'

[sys]
mem_limit = '128'

[sys.files]
bootrom_path = '$D/mcpx_1.0.bin'
flashrom_path = '$D/Cerbios.bin'
eeprom_path = '$EEPROM'
hdd_path = '$HDD'
dvd_path = '$ISO'
TOML
echo "h2sl-$i: TOML -> $TOML (H2 ISO, net bind :$BINDP remote :$REMP, eeprom $(basename "$EEPROM"), hdd $(basename "$HDD"))"

# ---- 3. launch xemu under sigshield (signals BLOCKED, ptrace shim) --------------
export DISPLAY="${DISPLAY:-:0}"
export SDL_VIDEODRIVER=x11
export XAUTHORITY="${XAUTHORITY:-$(ls -t /run/user/$(id -u)/xauth_* 2>/dev/null | head -1)}"
[ -f "$SHIM" ] && export LD_PRELOAD="$SHIM"
rm -f "$QMP" 2>/dev/null || true
setsid python3 "$T/sigshield.py" --pidfile "$PIDFILE" -- \
       xemu -config_path "$TOML" -display xemu -name "$NAME" \
            -qmp "unix:$QMP,server,nowait" \
       >"$XLOG" 2>&1 </dev/null &
sleep 0.5
echo "h2sl-$i: launched pid=$(cat "$PIDFILE" 2>/dev/null) qmp=$QMP log=$XLOG shim=${LD_PRELOAD:-none}"
