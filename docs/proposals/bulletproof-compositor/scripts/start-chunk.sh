#!/bin/bash
# starts one 2-hour soak chunk detached.  usage: start-chunk.sh N SEED [SECONDS] [BINARY]
# (new folder per chunk; nothing is deleted except an old runtime folder of the same chunk number)
ulimit -c 0
export W=${BC_WORK:?set BC_WORK to your work folder}
. $W/env.sh
N=$1; SEED=$2; SECS=${3:-7200}; BIN=${4:-$W/bin/driftwm-p25}
rm -rf --one-file-system $W/soak-c$N /tmp/bc4soak$N
python3 $W/launch.py soak-c$N SOAK_DISPLAY=:92 SOAK_RT=/tmp/bc4soak$N -- python3 $W/soak2.py $SECS $W/soak-c$N $SEED $BIN
date
