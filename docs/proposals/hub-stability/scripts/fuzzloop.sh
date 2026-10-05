#!/bin/bash
# usage: fuzzloop.sh BIN LABEL SEED_FROM SEED_TO SECONDS_PER_SEED
ulimit -c 0
D=${HS_WORK}/hs
BIN=$1; LABEL=$2; A=$3; B=$4; SECS=$5
SUM=/tmp/hs-fzloop-$LABEL.txt
: > $SUM
for seed in $(seq $A $B); do
  bash $D/fuzzrun.sh $BIN $SECS $seed /tmp/hs-fzl-$LABEL-$seed > /tmp/hs-fzl-$LABEL-$seed.out 2>&1
  r=$(grep -E "COMPOSITOR (ALIVE|DIED)" /tmp/hs-fzl-$LABEL-$seed.out | head -1)
  p=$(grep -A1 "panicked" /tmp/hs-fzl-$LABEL-$seed.out | head -2 | tr '\n' ' ' | cut -c1-300)
  m=$(grep -E "^done|DEAD" /tmp/hs-fzl-$LABEL-$seed.out | head -1); h=$(grep HEALTH /tmp/hs-fzl-$LABEL-$seed.out | head -1)
  echo "seed=$seed $r | $m | $h | $p" >> $SUM
done
echo FINISHED >> $SUM
