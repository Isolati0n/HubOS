#!/bin/bash
# same IPC fuzzer on p25, same seeds, but NO address-space limit (ULIM omitted), peak memory printed by ipcfuzz at the end
ulimit -c 0
B=${BC_WORK:?set BC_WORK to your work folder}
export IPCF_KEEP=3000
for s in 11 12 13; do
  rm -rf $B/ipcn$s
  bash $B/py.sh $B/ipcfuzz.py 240 $B/ipcn$s $s $B/bin/driftwm-p25 > $B/ipcn$s.out 2>&1
done
echo done > $B/ipcnolim.done
