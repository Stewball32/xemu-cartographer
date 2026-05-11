#!/bin/sh
# Clean stale Firefox profile locks left over from previous unclean shutdowns.
# A leftover .parentlock can confuse Firefox 145 into exiting status-0
# immediately on launch, producing an infinite supervisor respawn loop and a
# black-screen kiosk. -no-remote (set via FF_CUSTOM_ARGS in podman.go) is the
# primary mitigation; this script is belt-and-suspenders.
set -eu

PROFILE=/config/profile
[ -d "$PROFILE" ] || exit 0

for f in "$PROFILE/.parentlock" "$PROFILE/lock" "$PROFILE/sessionCheckpoints.json"; do
    if [ -e "$f" ]; then
        echo "[clear-firefox-locks] removing $f"
        rm -f "$f"
    fi
done
