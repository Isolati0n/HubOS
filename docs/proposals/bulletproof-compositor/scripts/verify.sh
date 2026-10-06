#!/bin/bash
# verify.sh BINNAME : runs every minimal reproduction against bin/BINNAME and prints one line each
B=${BC_WORK:?set BC_WORK to your work folder}
BIN=$B/bin/$1
echo "### NUL in config"; bash $B/nulcfg.sh $BIN 2>&1 | grep -E "==|exit status|panicked" | cut -c1-170
bash $B/nulstart.sh $BIN 2>&1 | grep -E "start-up|hot reload" | cut -c1-170
echo "### popup parent cycles"; bash $B/popall.sh $BIN v-$1 2>&1 | grep RESULT
echo "### subsurface place_above(self)"; bash $B/reprorun.sh $BIN vsub-$1 python3 $B/repro_subsurface.py SOCK 2>&1 | grep -E "RESULT|answer"
echo "### absurd geometry"; bash $B/reprorun.sh $BIN vvp-$1 python3 $B/repro_damage.py SOCK DPID viewport_dst_big 2>&1 | grep -E "RESULT"
bash $B/reprorun.sh $BIN vsp-$1 python3 $B/repro_damage.py SOCK DPID subpos_2147483647 2>&1 | grep -E "RESULT"
bash $B/reprorun.sh $BIN vsp2-$1 python3 $B/repro_damage.py SOCK DPID subpos_1073741824 2>&1 | grep -E "RESULT"
echo "### IPC: Move near the 32-bit limit, then a navigation action (BC-13)"; bash $B/ipcnav.sh $BIN vn-$1 2>&1 | grep -E "RESULT|panicked"
echo "### IPC: Resize of a stand-in to 32768 x 32768 (BC-12)"; RLIM_V=8000000 REPRO_OUT=$B/rr bash $B/reprorun.sh $BIN vsi-$1 python3 $B/repro_ipc.py SOCK standin_resize_32768 2>&1 | grep -E "RESULT|panicked"
