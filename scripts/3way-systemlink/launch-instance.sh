#!/usr/bin/env bash
# launch-instance.sh N  — bring up ONE of the 3 networked CE instances.
# Starts a hardened virtual pad (survives Bash teardown), builds the per-instance
# TOML (distinct MAC eeprom + own HDD + [net] udp through the reflector + QMP),
# and launches xemu under sigshield.py (teardown signals BLOCKED so xemu survives
# a Cowork Bash-call teardown — the thing that normally kills a detached xemu).
set -uo pipefail
i="${1:?usage: launch-instance.sh N (1|2|3)}"
D=/home/stew/.local/share/xemu/xemu
T=$D/3way
HARNESS_PAD=/home/stew/repos/xemu-cartographer-harness/scripts/runtime/padpool.py
ISO='/home/stew/repos/halo-offset-mapper/iso/Halo CE.iso'
SHIM=$D/cart-ptrace-shim.so

# ---- per-instance parameters -------------------------------------------------
case "$i" in
  1) PADBASE=0x04A0; EEPROM=$D/eeprom1.bin; HDD=$T/ovl-ce1.qcow2; BINDP=9971; REMP=9981;;
  2) PADBASE=0x04B0; EEPROM=$D/eeprom2.bin; HDD=$T/ovl-ce2.qcow2; BINDP=9972; REMP=9982;;
  3) PADBASE=0x04C0; EEPROM=$D/eeprom3.bin; HDD=$T/ovl-ce3.qcow2; BINDP=9973; REMP=9983;;
  *) echo "instance must be 1|2|3"; exit 2;;
esac
NAME=ce-3way-$i
RD=/run/user/$(id -u)/xemu-harness/3way-$i
QMP=/run/user/$(id -u)/xemu-qmp-3way-$i.sock
TOML=$T/xemu-3way-$i.toml
PIDFILE=$T/logs/xemu-$i.pid
XLOG=$T/logs/xemu-$i.out

# ---- 1. hardened pad (port1) -------------------------------------------------
# idempotent: clear stale rundir + any stale pad for THIS instance
pkill -KILL -f "[p]adpool.py --count .* --rundir $RD" 2>/dev/null || true
rm -rf "$RD"; mkdir -p "$RD" "$T/logs"
setsid python3 "$HARNESS_PAD" --count 1 --rundir "$RD" --base-version "$PADBASE" \
       >"$RD/padpool.out" 2>&1 </dev/null &
for _ in $(seq 1 50); do [ -f "$RD/pads.json" ] && break; sleep 0.2; done
[ -f "$RD/pads.json" ] || { echo "instance $i: padpool failed"; cat "$RD/padpool.out"; exit 1; }
GUID=$(python3 -c "import json;print([p['guid'] for p in json.load(open('$RD/pads.json'))['pads'] if p['port']==1][0])")
echo "instance $i: pad guid=$GUID"

# ---- 2. build the TOML -------------------------------------------------------
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
echo "instance $i: TOML -> $TOML  (net bind :$BINDP remote :$REMP, eeprom $(basename "$EEPROM"), hdd $(basename "$HDD"))"

# ---- 3. launch xemu under sigshield (signals BLOCKED, new session) -----------
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
echo "instance $i: launched xemu DISPLAY=$DISPLAY XAUTH=${XAUTHORITY:-<none>} pid=$(cat "$PIDFILE" 2>/dev/null) qmp=$QMP log=$XLOG"
