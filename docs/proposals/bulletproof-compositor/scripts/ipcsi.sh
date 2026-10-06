#!/bin/bash
# usage: ipcsi.sh BIN LABEL    (IPC Resize of a STAND-IN to a huge size)
B=${BC_WORK:?set BC_WORK to your work folder}
export REPRO_OUT=$B/rr
for v in standin_resize_65535 standin_resize_32768 standin_resize_20000 standin_resize_16384 standin_resize_4000; do
  RLIM_V=8000000 bash $B/reprorun.sh $1 $2-$v python3 $B/repro_ipc.py SOCK $v 2>&1 | grep -E "RESULT|->|panicked|capacity|memory alloc" | cut -c1-150 | tr '\n' ' '; echo
done
