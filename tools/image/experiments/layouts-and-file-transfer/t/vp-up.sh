#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
W=$LFT
. $W/env.sh
export XDG_RUNTIME_DIR=/tmp/dwx PYLIB=$W/pylib WAYLAND_DISPLAY=wayland-1
rm -f $W/t/vp.fifo; mkfifo $W/t/vp.fifo
python3 $W/vptr.py $W/t/vp.fifo > $W/t/vp.out 2>&1 &
echo $! > $W/t/vp.pid
sleep 2; cat $W/t/vp.out
