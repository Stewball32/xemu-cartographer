#!/usr/bin/env bash
# cap.sh N OUTFILE  — capture instance N's xemu window (by _NET_WM_PID) to OUTFILE.
# Bash-driven (xdotool+import), deterministic, no computer-use. Maps the shared
# 'xemu | v0.8.136' window title to the right instance via the launch pidfile.
set -uo pipefail
i="${1:?usage: cap.sh N OUTFILE}"; out="${2:?need outfile}"
export DISPLAY=:0 XAUTHORITY="${XAUTHORITY:-$(ls -t /run/user/1000/xauth_* 2>/dev/null | head -1)}"
want=$(cat /home/stew/.local/share/xemu/xemu/3way/logs/xemu-$i.pid 2>/dev/null)
[ -n "$want" ] || { echo "no pid for instance $i"; exit 1; }
for id in $(xdotool search --name "xemu | v0.8.136" 2>/dev/null); do
  wpid=$(xdotool getwindowpid "$id" 2>/dev/null)
  if [ "$wpid" = "$want" ]; then
    import -window "$id" "$out" 2>/dev/null && { echo "captured instance $i (win $id pid $wpid) -> $out"; exit 0; }
  fi
done
echo "instance $i window not found (pid $want)"; exit 1
