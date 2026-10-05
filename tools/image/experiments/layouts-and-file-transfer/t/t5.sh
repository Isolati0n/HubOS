#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 5: implicit grab and real Wayland drag and drop between two clients under driftwm (nested on headless sway).
# A = drag source (red), B = drop target (blue). The pointer is injected into sway's seat with `swaymsg seat - cursor ...`.
. $LFT/t/dw.sh
[ -n "$DWBIN" ] && DW=$DWBIN
export PYLIB=$W/pylib
export PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1)
export SWAYSOCK=$(ls $XDG_RUNTIME_DIR/sway-ipc.*.sock | head -1)
cd /tmp
rm -rf $T/state/* /tmp/dwx/driftwm /tmp/dwx/wayland-[2-9]*
start_dw $T/cfg-default.toml
echo "display=$WAYLAND_DISPLAY"
python3 $W/dndclient.py ${ROLE_A:-source} term-a ff0000 ${SECS:-600} $T/cli-a.log > $T/cli-a.out 2>&1 &
echo $! > $T/cli-a.pid
python3 $W/dndclient.py target term-b 0000ff ${SECS:-600} $T/cli-b.log > $T/cli-b.out 2>&1 &
echo $! > $T/cli-b.pid
sleep 4
dw msg state | sed -n 4,8p
# A at canvas (-300,0) 400x300, B at (300,0) 400x300 (centres, Y up); camera centre (cx,cy)
IDA=$(dw msg --json state | jq -r '..|objects|select(.app_id=="term-a")|.id' | head -1)
IDB=$(dw msg --json state | jq -r '..|objects|select(.app_id=="term-b")|.id' | head -1)
echo "ids: A=$IDA B=$IDB"
dw msg move -300 0 --id $IDA >/dev/null; dw msg resize 400 300 --id $IDA >/dev/null
dw msg move 300 0 --id $IDB >/dev/null; dw msg resize 400 300 --id $IDB >/dev/null
dw msg camera 0 0 >/dev/null; sleep 2
dw msg state | sed -n 1,8p
echo "--- which client is which window (ids): "; head -3 $T/cli-a.log $T/cli-b.log
