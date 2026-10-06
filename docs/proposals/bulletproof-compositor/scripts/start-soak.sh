#!/bin/bash
# starts the 12 h soak detached
ulimit -c 0
export W=${BC_WORK:?set BC_WORK to your work folder}
. $W/env.sh
rm -rf $W/soak12h /tmp/bc4soak
python3 $W/launch.py soak12h SOAK_DISPLAY=:92 SOAK_RT=/tmp/bc4soak -- python3 $W/soak2.py ${1:-43200} $W/soak12h 20261005 $W/bin/driftwm-p22
date
