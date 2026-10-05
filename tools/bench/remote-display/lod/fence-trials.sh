#!/bin/bash
# Does a client Fence message, sent while the client still has an update request outstanding, crash wayvnc?
# Repeats the same test N times (default 8), each time on a fresh node, and counts how often wayvnc died.
#   BENCH_TMP=/tmp/bench ./fence-trials.sh [N] [PHASES] [FENCE_FLAGS]
set -u
ulimit -c 0
cd "$(dirname "$0")"
export BENCH_TMP=${BENCH_TMP:-/tmp/bench}
N=${1:-8}; PH=${2:-normal:2,fence:3}; FF=${3:-1}
crashed=0
for i in $(seq "$N"); do
  ./node.sh start scroll 1280 720 5901 -Ldebug > $BENCH_TMP/lod/p.txt || exit 1
  sleep 2
  WP=$(awk '$1=="wayvnc"{print $2}' $BENCH_TMP/lod/p.txt)
  python3 rfbprobe.py --port 5901 --fence-flags "$FF" --phases "$PH" > $BENCH_TMP/lod/out.txt 2>&1
  sleep 0.5
  if grep -a -q 'is_blocked_by_fence' $BENCH_TMP/lod/wayvnc.log; then r=CRASHED; crashed=$((crashed+1)); else r=ok; fi
  echo "trial $i: $r   $(grep -a -m1 'Assertion' $BENCH_TMP/lod/wayvnc.log | cut -c1-120)"
  ./node.sh stop
done
echo "phases=$PH fence-flags=$FF  wayvnc aborted in $crashed of $N trials"
