#!/bin/bash
# usage: py.sh script.py args...   (sets W, ulimit -c 0, nice)
ulimit -c 0
export W=${BC_WORK:?set BC_WORK to your work folder}
. $W/env.sh
export FUZZ_AVOID_CYCLE=${FUZZ_AVOID_CYCLE:-}
cd $W
exec nice -n ${NICE:-15} python3 "$@"
