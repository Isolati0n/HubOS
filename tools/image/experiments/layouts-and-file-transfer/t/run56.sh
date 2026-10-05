#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# run56.sh ROLE_A [DWBIN]: fresh driftwm + two clients, then the press-move-release (t6.sh)
T=$LFT/t
export ROLE_A=$1
[ -n "$2" ] && export DWBIN=$2
for p in dw cli-a cli-b; do [ -f $T/$p.pid ] && kill $(cat $T/$p.pid) 2>/dev/null; done
sleep 1
bash $T/t5.sh 2>&1 | tail -9
echo "######## t6 with A role=$ROLE_A"
bash $T/t6.sh 2>&1
