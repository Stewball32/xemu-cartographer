#!/usr/bin/env bash
# launch-h2.sh  — bring up ONE Halo 2 xemu instance (tag "h2-1"), splitscreen-ready.
# Reuses the CE 3way rig tooling verbatim: a hardened 2-pad padpool (port1+port2 so
# splitscreen custom games can start), the per-instance TOML builder, sigshield
# survival wrapper (teardown signals BLOCKED), the LD_PRELOAD PR_SET_PTRACER shim
# (so /proc/<pid>/mem is readable under yama ptrace_scope=1), and a QMP socket.
# Naming is aligned to the 3way-$N scheme (N=h2-1) so cap.sh / nav.sh / pad FIFOs
# work unchanged:  cap.sh h2-1 out.png  |  FIFO .../3way-h2-1/p1.fifo,p2.fifo.
set -uo pipefail
i=h2-1
D=/home/stew/.local/share/xemu/xemu
T=$D/3way
HARNESS_PAD=/home/stew/repos/xemu-cartographer-harness/scripts/runtime/padpool.py
ISO='/home/stew/repos/halo-offset-mapper/iso/Halo 2.iso'
SHIM=$D/cart-ptrace-shim.so

PADBASE=0x0500          # distinct GUID namespace (CE 3way used 0x04A0/B0/C0)
EEPROM=$D/eeprom1.bin
HDD=$T/ovl-h2-1.qcow2
BINDP=9974; REMP=9984   # own net ports; no reflector needed for local splitscreen
NAME=h2-3way-$i
RD=/run/user/$(id -u)/xemu-harness/3way-$i
QMP=/run/user/$(id -u)/xemu-qmp-3way-$i.sock
TOML=$T/xemu-3way-$i.toml
PIDFILE=$T/logs/xemu-$i.pid
XLOG=$T/logs/xemu-$i.out

# ---- 1. hardened pad pool (port1 + port2 for splitscreen) --------------------
pkill -KILL -f "[p]adpool.py --count .* --rundir $RD" 2>/dev/null || true
rm -rf "$RD"; mkdir -p "$RD" "$T/logs"
setsid python3 "$HARNESS_PAD" --count 2 --rundir "$RD" --base-version "$PADBASE" \
       >"$RD/padpool.out" 2>&1 </dev/null &
for _ in $(seq 1 50); do [ -f "$RD/pads.json" ] && break; sleep 0.2; done
[ -f "$RD/pads.json" ] || { echo "instance $i: padpool failed"; cat "$RD/padpool.out"; exit 1; }
GUID1=$(python3 -c "import json;print([p['guid'] for p in json.load(open('$RD/pads.json'))['pads'] if p['port']==1][0])")
GUID2=$(python3 -c "import json;print([p['guid'] for p in json.load(open('$RD/pads.json'))['pads'] if p['port']==2][0])")
echo "instance $i: pad1 guid=$GUID1  pad2 guid=$GUID2"

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
  { gamepad_id = '$GUID1' },
  { gamepad_id = '$GUID2' },
]

[input.bindings]
port1_driver = 'usb-xbox-gamepad'
port1 = '$GUID1'
port2_driver = 'usb-xbox-gamepad'
port2 = '$GUID2'

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
echo "instance $i: TOML -> $TOML (H2 ISO, net :$BINDP/:$REMP, hdd $(basename "$HDD"))"

# ---- 3. launch xemu under sigshield (signals BLOCKED, ptrace shim) -----------
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
echo "instance $i: launched pid=$(cat "$PIDFILE" 2>/dev/null) qmp=$QMP log=$XLOG shim=${LD_PRELOAD:-none}"
