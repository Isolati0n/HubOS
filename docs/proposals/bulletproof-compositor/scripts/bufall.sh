#!/bin/bash
B=${BC_WORK:?set BC_WORK to your work folder}
for v in buf_65536x1 buf_1048576x1 buf_16777216x1 buf_134217728x1 buf_1x16777216 buf_1x134217728 buf_32768x32768; do
  RLIM_V=6000000 bash $B/reprorun.sh $1 $2-$v python3 $B/repro_damage.py SOCK DPID $v 2>&1 | grep -E "RESULT|smashing|panicked" | tr '\n' ' '; echo
done
