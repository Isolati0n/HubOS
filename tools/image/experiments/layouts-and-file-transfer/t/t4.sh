#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 4: DEFAULT config (all [session] flags off) + --session-file.  One window is suspended by hand, one stays live.
# kill -9, restart: which come back?  Then relaunch the stand-in.   DW can be overridden (DW=release binary).
. $LFT/t/dw.sh
[ -n "$DWBIN" ] && DW=$DWBIN
export PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1)
cd /tmp
SF=$T/state/driftwm/session.json
rm -rf $T/state/* /tmp/dwx/driftwm /tmp/dwx/wayland-[2-9]*
start_dw $T/cfg-default.toml --session-file $SF
echo "display=$WAYLAND_DISPLAY binary=$DW"
foot --app-id=term-a --title=First sleep 1500 >/dev/null 2>&1 &
foot --app-id=term-b --title=Second sleep 1500 >/dev/null 2>&1 &
sleep 3
dw msg move 300 200 --id 0 >/dev/null; dw msg resize 640 480 --id 0 >/dev/null
dw msg move -500 -300 --id 1 >/dev/null; dw msg resize 500 350 --id 1 >/dev/null
echo "--- suspend term-a (explicit)"; dw msg suspend term-a; sleep 3
dw msg state | sed -n 4,8p
sleep 3
echo "--- session.json (default flags)"; jq -c '.entries[] | {app_id,origin,position,size,focused}' $SF
echo "--- KILL -9"; kill -9 $(cat $T/dw.pid); sleep 2; pgrep -a foot | grep -v hubos-s | cut -c1-60; echo "(foot list end)"
rm -f /tmp/dwx/wayland-2 /tmp/dwx/wayland-2.lock
start_dw $T/cfg-default.toml --session-file $SF
echo "--- after restart"; dw msg state | sed -n 1,8p
echo "--- relaunch term-a"; dw msg relaunch term-a; sleep 4
dw msg state | sed -n 4,8p
echo "--- alive?"; ps -p $(cat $T/dw.pid) -o pid= || echo "DRIFTWM DEAD"; clean_dw | grep -A2 panicked | cut -c1-200
