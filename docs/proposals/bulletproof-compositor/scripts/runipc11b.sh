#!/bin/bash
ulimit -c 0
B=${BC_WORK:?set BC_WORK to your work folder}
export IPCF_KEEP=100000
rm -rf $B/ipcf11b
bash $B/py.sh $B/ipcfuzz.py 240 $B/ipcf11b 11 $B/bin/driftwm-p25 6000000 > $B/ipcf11b.out 2>&1
