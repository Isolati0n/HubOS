#!/bin/bash
# usage: ipcall.sh BIN LABEL
B=${BC_WORK:?set BC_WORK to your work folder}
export REPRO_OUT=$B/rr
for v in shot_neg_scale shot_zero_scale shot_region_h0_negscale shot_region_zero shot_region_neg shot_region_h0 shot_huge_scale; do
  RLIM_V=8000000 bash $B/reprorun.sh $1 $2-$v python3 $B/repro_ipc.py SOCK $v 2>&1 | grep -E "RESULT|->|panicked|capacity" | cut -c1-200 | tr '\n' ' '; echo
done
