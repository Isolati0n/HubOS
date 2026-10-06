#!/bin/bash
B=${BC_WORK:?set BC_WORK to your work folder}
for v in vpdst_4096 vpdst_16384 vpdst_32768 vpdst_65536 vpdst_1048576 vpdst_16777216 vpdst_268435456; do
  bash $B/reprorun.sh $1 $2-$v python3 $B/repro_damage.py SOCK DPID $v 2>&1 | grep -E "RESULT|smashing" | tr '\n' ' '; echo
done
