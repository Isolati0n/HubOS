#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 6: press in A, move to B, release (pointer injected with a zwlr_virtual_pointer into the sway parent).
# Needs driftwm + the two clients from t5.sh and vptr from vp-up.sh.
. $LFT/t/dw.sh
export WAYLAND_DISPLAY=${WAYLAND_DISPLAY:-wayland-2}
P() { echo "$@" > $T/vp.fifo; }
P "abs 340 420"; sleep 0.7
N1=$(wc -l < $T/cli-a.log); N2=$(wc -l < $T/cli-b.log)
P "press"; sleep 0.5
for x in 400 500 600 700 800 900 940; do P "abs $x 420"; sleep 0.3; done
sleep 0.5
echo "--- driftwm focus while the button is still held:"; dw msg state | sed -n 4,6p
P "release"; sleep 1.5
echo "--- A log since press"; tail -n +$((N1+1)) $T/cli-a.log
echo "--- B log since press"; tail -n +$((N2+1)) $T/cli-b.log
