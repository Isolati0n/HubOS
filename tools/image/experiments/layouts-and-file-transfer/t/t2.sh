#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 2: restart after the kill -9 of t1.sh; what comes back?  Then relaunch one stand-in.
. $LFT/t/dw.sh
export PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1)
cd /tmp
SF=$T/state/driftwm/session.json
CFG=${1:-$T/cfg-on.toml}
rm -f /tmp/dwx/wayland-2 /tmp/dwx/wayland-2.lock
echo "--- stale files before restart"; ls /tmp/dwx/driftwm
start_dw $CFG --session-file $SF
echo "display=$WAYLAND_DISPLAY"
echo "--- warnings in log"; clean_dw | grep -E 'WARN|ERROR' | grep -vE 'EGL|xkb|cosmic|xdg_toplevel_icon|dbus|bus|xwayland' | cut -c1-220
sleep 1
echo "--- state right after restart (no app started)"; dw msg state
echo "--- json for windows"; dw msg --json state | jq -c '.Ok.State.windows // .windows // .' 2>/dev/null | head -c 1500; echo
sleep 3
echo "--- pgrep foot (anything auto-launched?)"; pgrep -a foot | grep -v hubos-s | cut -c1-80; echo "(end)"
echo "--- session.json now"; jq -c . $SF
