#!/bin/bash
ulimit -c 0
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
BIN=${HSBIN:-$W/bin/hsfuzz-patched}
OUT=$W/hsf-results; mkdir -p $OUT; cd $OUT
SECS=${1:-150}
for t in toml parse json geom; do
  nice -n 15 $BIN $t $SECS ${2:-1} $OUT/$t 2>&1 | tee $OUT/$t-summary.txt | head -30
done
