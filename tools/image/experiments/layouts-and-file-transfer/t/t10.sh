#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 10: can an overlay mapped DURING a held-button drag learn where the button was released?
# (needs: sway parent + vptr from vp-up.sh; starts driftwm + clients A(plain) and B itself)
export ROLE_A=plain
T=$LFT/t
bash $T/t5.sh > /dev/null 2>&1
. $T/dw.sh
export PYLIB=$W/pylib
export WAYLAND_DISPLAY=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | grep -v "^$PARENT$" | tail -1)
P() { echo "$@" > $T/vp.fifo; }
dw msg state | sed -n 4,6p
P "abs 340 420"; sleep 0.7
N1=$(wc -l < $T/cli-a.log); N2=$(wc -l < $T/cli-b.log)
P "press"; sleep 0.5
echo "--- button held in A; now mapping the overlay"
python3 $W/overlay.py oneshot 20 $T/overlay.log > $T/overlay.out 2>&1 &
sleep 2
cat $T/overlay.log
for x in 500 700 900 940; do P "abs $x 420"; sleep 0.4; done
echo "--- overlay log after moves (button still held):"; cat $T/overlay.log
P "release"; sleep 2
echo "--- overlay log after release"; cat $T/overlay.log
echo "--- A log since press"; tail -n +$((N1+1)) $T/cli-a.log
echo "--- B log since press"; tail -n +$((N2+1)) $T/cli-b.log
echo "--- layers left on driftwm:"; dw msg state | grep -E "^layers"
cat $T/overlay.out | tail -3
