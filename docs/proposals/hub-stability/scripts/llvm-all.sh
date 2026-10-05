#!/bin/bash
ulimit -c 0
D=${HS_WORK}
cd /tmp
OUT=/tmp/hs-llvm-summary.txt
: > $OUT
for v in min default blur shader heavy; do
  if [ "$v" = default ]; then export HOG=1; else unset HOG; fi
  echo "=== variant $v ($(date +%T), load $(cut -d' ' -f1-3 /proc/loadavg))" >> $OUT
  python3 $D/hs/llvm.py /tmp/hs-llvm-$v $v >> $OUT 2>&1
done
unset HOG
echo "=== variant min with LP_NUM_THREADS=2 ($(date +%T), load $(cut -d' ' -f1-3 /proc/loadavg))" >> $OUT
python3 $D/hs/llvm.py /tmp/hs-llvm-min-lp2 min 2 >> $OUT 2>&1
echo FINISHED >> $OUT
