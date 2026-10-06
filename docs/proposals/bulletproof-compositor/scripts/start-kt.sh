#!/bin/bash
# starts a private Xvfb :93 and the kill -9 test (CYCLES BIN LABEL) detached
ulimit -c 0
export W=${BC_WORK:?set BC_WORK to your work folder}
. $W/env.sh
CYC=${1:-3000}; BIN=${2:-$W/bin/driftwm-p20}; LABEL=${3:-kt}
[ -f $W/xvfb93.pid ] && kill -0 $(cat $W/xvfb93.pid) 2>/dev/null || python3 $W/launch.py xvfb93 -- bash $W/xvfb.sh :93 1280x720x24
sleep 3
rm -rf $W/$LABEL /tmp/bc4kt-$LABEL
python3 $W/launch.py $LABEL SOAK_DISPLAY=:93 KT_RT=/tmp/bc4kt-$LABEL -- python3 $W/killtest.py $CYC $W/$LABEL 77 $BIN
date
