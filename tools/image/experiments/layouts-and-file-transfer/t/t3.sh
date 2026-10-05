#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 3: on the restored stand-ins (run after t2.sh): bookmarks, relaunch, suspend_on_close, dismiss.
. $LFT/t/dw.sh
cd /tmp
SF=$T/state/driftwm/session.json
export WAYLAND_DISPLAY=wayland-2
echo "--- bookmarks after restart (restore_bookmarks=true)"; dw msg bookmark
echo "--- relaunch term-a (IPC)"; dw msg relaunch term-a; sleep 3
dw msg state | sed -n 4,8p
echo "--- relaunched app command line"; pgrep -a foot | grep -v hubos-s | cut -c1-80
echo "--- close term-a window (client-initiated: kill the foot)"; kill $(pgrep -f 'foot --app-id=term-a --title=First') ; sleep 2
dw msg state | sed -n 4,8p
echo "--- session.json entries"; jq -c '.entries[] | {app_id,origin,position,size}' $SF
echo "--- msg close on the stand-in (dismiss)"; dw msg close --id 0; sleep 2
dw msg state | sed -n 4,8p
sleep 2; jq -c '.entries[] | {app_id,origin,position,size}' $SF
