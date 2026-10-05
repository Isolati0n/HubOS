#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 1: save, then kill -9, with all four [session] flags on. (Restart is t2.sh.)
. $LFT/t/dw.sh
export PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1)
cd /tmp
SF=$T/state/driftwm/session.json
rm -rf $T/state/*
start_dw $T/cfg-on.toml --session-file $SF
echo "parent=$PARENT driftwm display=$WAYLAND_DISPLAY"
foot --app-id=term-a --title=First sleep 1500 >/dev/null 2>&1 &
echo $! > $T/foot-a.pid
foot --app-id=term-b --title=Second sleep 1500 >/dev/null 2>&1 &
echo $! > $T/foot-b.pid
foot --app-id=nodesk --title=NoDesktopEntry sleep 1500 >/dev/null 2>&1 &
echo $! > $T/foot-c.pid
sleep 3
echo "--- state after launch"; dw msg state
IDA=$(dw msg --json state | jq -r '..|objects|select(.app_id=="term-a")|.id' | head -1)
IDB=$(dw msg --json state | jq -r '..|objects|select(.app_id=="term-b")|.id' | head -1)
IDC=$(dw msg --json state | jq -r '..|objects|select(.app_id=="nodesk")|.id' | head -1)
dw msg move 300 200 --id $IDA
dw msg resize 640 480 --id $IDA
dw msg move -500 -300 --id $IDB
dw msg resize 500 350 --id $IDB
dw msg move 0 -600 --id $IDC
dw msg camera 120 80
sleep 2
dw msg zoom 0.8
sleep 2
dw msg bookmark myplace 700 -400
sleep 8
echo "--- state before kill"; dw msg state
echo "--- session.json before kill"; jq -c . $SF; ls -la $T/state/driftwm
echo "--- KILL -9 compositor pid $(cat $T/dw.pid)"
kill -9 $(cat $T/dw.pid)
sleep 2
echo "--- after kill: foot alive?"; pgrep -a foot | grep -v hubos-s | cut -c1-80
echo "--- files left"; ls -la $T/state/driftwm $XDG_RUNTIME_DIR $XDG_RUNTIME_DIR/driftwm 2>&1
cp $SF $T/session-after-kill.json
echo "--- stale ipc socket test"; dw msg state 2>&1 | head -3
