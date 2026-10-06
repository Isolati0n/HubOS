#!/bin/bash
# usage: ipcnav.sh BIN LABEL    (BC-13: IPC Move to a coordinate near the 32-bit limit, then a navigation action)
B=${BC_WORK:?set BC_WORK to your work folder}
export REPRO_OUT=$B/rr
RLIM_V=8000000 bash $B/reprorun.sh $1 $2-nav python3 $B/repro_ipc.py SOCK nav_far 2>&1 | grep -E "RESULT|->|panicked|min > max" | cut -c1-170
