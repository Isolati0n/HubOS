#!/bin/bash
# usage: collectchunk.sh N   : copies the small result files of soak chunk N into the repository results folder
ulimit -c 0
B=${BC_WORK:?set BC_WORK to your work folder}
R=${REPO:?set REPO to the repository}/docs/proposals/bulletproof-compositor/results
N=$1
D=$B/soak-c$N
[ -d $D ] || { echo "no chunk $N"; exit 1; }
cp $D/samples.csv $R/soak-c$N-samples.csv
cp $D/stats.json $R/soak-c$N-stats.json
grep -v "debug-counters" $D/events.log | cut -c1-400 > $R/soak-c$N-events.log
grep "debug-counters" $D/events.log | cut -c1-1200 > $R/soak-c$N-debug-counters.log
ls -l $R | grep "soak-c$N" | awk '{print $5, $9}'
