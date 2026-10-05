#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 13: ARMED overlay.  The overlay is mapped BEFORE the button goes down; press in A's place, move, release over B.
# (needs the sway parent, vptr from vp-up.sh; starts driftwm + clients itself)
export ROLE_A=plain
T=$LFT/t
bash $T/t5.sh > /dev/null 2>&1
. $T/dw.sh
export PYLIB=$W/pylib
export WAYLAND_DISPLAY=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | grep -v "^$PARENT$" | tail -1)
P() { echo "$@" > $T/vp.fifo; }
P "abs 340 420"; sleep 0.7
N1=$(wc -l < $T/cli-a.log); N2=$(wc -l < $T/cli-b.log)
python3 $W/overlay.py stay 12 $T/overlay.log > $T/overlay.out 2>&1 &
sleep 2
P "abs 345 425"; sleep 0.5
P "press"; sleep 0.4
for x in 500 700 900 940; do P "abs $x 420"; sleep 0.3; done
P "release"; sleep 1
echo "--- overlay log"; cat $T/overlay.log
echo "--- A log since arming"; tail -n +$((N1+1)) $T/cli-a.log
echo "--- B log since arming"; tail -n +$((N2+1)) $T/cli-b.log
echo "--- window rects for the lookup (id, centre Y-up, size) and camera/zoom:"; dw msg --json state | jq -c '.. | objects | select(.app_id? != null and .position? != null) | {app_id,position,size}'; dw msg state | grep -E "^(camera|zoom)" | tr '\n' ' '; echo
