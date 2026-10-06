#!/bin/bash
ulimit -c 0
B=${BC_WORK:?set BC_WORK to your work folder}
export IPCMIN_ULIM=6000000
bash $B/py.sh $B/ipcmin.py $B/bin/driftwm-p25 $B/ipcf11b/ipc-crash-seed11.json $B/ipcmin3 > $B/ipcmin3.out 2>&1
