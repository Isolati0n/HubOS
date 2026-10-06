#!/bin/bash
# usage: ipcfar.sh BIN LABEL
B=${BC_WORK:?set BC_WORK to your work folder}
export REPRO_OUT=$B/rr
for v in two_far_max two_far_1e9 two_far_1e6; do
  RLIM_V=8000000 bash $B/reprorun.sh $1 $2-$v python3 $B/repro_ipc.py SOCK $v 2>&1 | grep -E "RESULT|->|panicked|overflow|capacity" | cut -c1-170 | tr '\n' ' '; echo
done
