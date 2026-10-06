#!/bin/bash
# usage: ipcrs.sh BIN LABEL    (IPC Resize to a huge size of a LIVE window; the minimal crash from ipcmin was on a stand-in, see ipcmin1)
B=${BC_WORK:?set BC_WORK to your work folder}
export REPRO_OUT=$B/rr
for v in resize_big_live_65535 resize_big_live_32768 resize_big_live_20000 resize_big_live_16384; do
  RLIM_V=8000000 bash $B/reprorun.sh $1 $2-$v python3 $B/repro_ipc.py SOCK $v 2>&1 | grep -E "RESULT|->|panicked|capacity" | cut -c1-150 | tr '\n' ' '; echo
done
