#!/bin/bash
# usage: fzloop.sh BIN LABEL SEED_FROM SEED_TO SECS
ulimit -c 0
B=${BC_WORK:?set BC_WORK to your work folder}
SUM=$B/fz/summary-$2.txt; : > $SUM
for seed in $(seq $3 $4); do
  bash $B/fuzzone.sh $1 $5 $seed $B/fz/$2-$seed > $B/fz/$2-$seed.out 2>&1
  echo "seed=$seed | $(grep -E 'COMPOSITOR (ALIVE|DIED|HUNG)' $B/fz/$2-$seed.out | head -2 | tr '\n' ' ') | $(grep -E '^done' $B/fz/$2-$seed.out | head -1) | $(grep -E 'POST-FUZZ|SIGTERM' $B/fz/$2-$seed.out | head -2 | tr '\n' ' ') | $(grep -m1 -A1 panicked $B/fz/$2-$seed/dw.log | tr '\n' ' ' | cut -c1-250)" >> $SUM
done
echo FINISHED >> $SUM
