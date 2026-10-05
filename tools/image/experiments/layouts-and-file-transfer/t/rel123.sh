#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# t1 + t2 + t3 with the RELEASE binary
T=$LFT/t
export DWBIN=$LFT/target-rel/release/driftwm
rm -rf /tmp/dwx/driftwm /tmp/dwx/wayland-[2-9]*
echo "######## T1 (save, kill -9)"; bash $T/t1.sh 2>&1
echo "######## T2 (restart)"; bash $T/t2.sh 2>&1
echo "######## T3 (relaunch etc.)"; bash $T/t3.sh 2>&1
