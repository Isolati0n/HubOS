#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 7: two windows of ONE app_id (like two Moonlight windows for two machines).  Save, kill -9, restart, relaunch both.
. $LFT/t/dw.sh
export PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1)
cd /tmp
SF=$T/state/driftwm/session.json
rm -rf $T/state/* /tmp/dwx/driftwm /tmp/dwx/wayland-[2-9]*
start_dw $T/cfg-on.toml --session-file $SF
foot --app-id=viewer --title="ai-1 - viewer" sleep 1500 >/dev/null 2>&1 &
sleep 2
foot --app-id=viewer --title="nas - viewer" sleep 1500 >/dev/null 2>&1 &
sleep 3
dw msg --json state | jq -c '.. | objects | select(.app_id=="viewer") | {id,title,position,size}'
IDS=$(dw msg --json state | jq -r '..|objects|select(.app_id=="viewer")|.id')
set -- $IDS
dw msg move -400 0 --id $1 >/dev/null; dw msg move 400 0 --id $2 >/dev/null
echo "titles by id:"; dw msg --json state | jq -c '.. | objects | select(.app_id=="viewer") | {id,title,position}'
sleep 3; echo "--- session.json"; jq -c '.entries[]' $SF
kill -9 $(cat $T/dw.pid); sleep 2
rm -f /tmp/dwx/wayland-2 /tmp/dwx/wayland-2.lock
start_dw $T/cfg-on.toml --session-file $SF
echo "--- after restart"; dw msg --json state | jq -c '.. | objects | select(.app_id=="viewer") | {id,title,position,suspended}'
echo "--- relaunch viewer (selector resolves to one stand-in), twice"
dw msg relaunch viewer; sleep 3; dw msg relaunch viewer; sleep 3
dw msg --json state | jq -c '.. | objects | select(.app_id=="viewer") | {id,title,position,suspended}'
