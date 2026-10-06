#!/bin/bash
# usage: ipcmove.sh BIN LABEL
B=${BC_WORK:?set BC_WORK to your work folder}
export REPRO_OUT=$B/rr
for v in move_then_shot_all move_then_shot_all_1e9 move_then_shot_all_1e6; do
  RLIM_V=8000000 bash $B/reprorun.sh $1 $2-$v python3 $B/repro_ipc.py SOCK $v 2>&1 | grep -E "RESULT|->|panicked|overflow|capacity" | cut -c1-200 | tr '\n' ' '; echo
done
