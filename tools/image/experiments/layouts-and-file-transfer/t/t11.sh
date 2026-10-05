#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 11: kill -9 then restart WITHOUT cleaning the runtime folder: what does a second start do with the stale files?
. $LFT/t/dw.sh
export PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1)
cd /tmp
SF=$T/state/driftwm/session.json
rm -rf $T/state/* /tmp/dwx/driftwm /tmp/dwx/wayland-[2-9]*
start_dw $T/cfg-on.toml --session-file $SF
echo "first start: display=$WAYLAND_DISPLAY"
foot --app-id=term-a --title=First sleep 1500 >/dev/null 2>&1 &
sleep 3
kill -9 $(cat $T/dw.pid); sleep 1
echo "after kill -9:"; ls /tmp/dwx /tmp/dwx/driftwm | tr '\n' ' '; echo
echo "second start, no cleanup, same WAYLAND_DISPLAY name wanted:"
env WAYLAND_DISPLAY=$PARENT RUST_LOG=info $DW --backend winit --config $T/cfg-on.toml --session-file $SF > $T/dw2.log 2>&1 &
echo $! > $T/dw.pid; sleep 4
sed -E 's/\x1b\[[0-9;]*m//g' $T/dw2.log | grep -E "Listening|IPC|socket|Created new" | cut -c1-200
ls /tmp/dwx /tmp/dwx/driftwm | tr '\n' ' '; echo
D=$(sed -E 's/\x1b\[[0-9;]*m//g' $T/dw2.log | grep -oE 'Listening on WAYLAND_DISPLAY=[a-z0-9-]+' | cut -d= -f2)
WAYLAND_DISPLAY=$D $DW msg state | sed -n 4,6p
kill $(cat $T/dw.pid)
