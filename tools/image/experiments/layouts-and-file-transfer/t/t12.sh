#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 12: setting camera and zoom back to back over the socket: which order sticks?
. $LFT/t/dw.sh
export PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1)
cd /tmp
rm -rf /tmp/dwx/driftwm /tmp/dwx/wayland-[2-9]*
start_dw $T/cfg-default.toml
foot --app-id=term-a --title=First sleep 1500 >/dev/null 2>&1 &
foot --app-id=term-b --title=Second sleep 1500 >/dev/null 2>&1 &
sleep 3
IDB=$(dw msg --json state | jq -r '..|objects|select(.app_id=="term-b")|.id' | head -1)
dw msg move 900 0 --id $IDB > /dev/null; sleep 1
for order in "camera-then-zoom" "zoom-then-camera" "zoom-wait-camera" "camera-wait-zoom"; do
  dw msg camera 0 0 >/dev/null; dw msg zoom 1 >/dev/null; sleep 2
  if [ $order = camera-then-zoom ]; then dw msg camera 300 -200 >/dev/null; dw msg zoom 0.8 >/dev/null; elif [ $order = zoom-then-camera ]; then dw msg zoom 0.8 >/dev/null; dw msg camera 300 -200 >/dev/null; elif [ $order = zoom-wait-camera ]; then dw msg zoom 0.8 >/dev/null; sleep 2; dw msg camera 300 -200 >/dev/null; else dw msg camera 300 -200 >/dev/null; sleep 2; dw msg zoom 0.8 >/dev/null; fi
  sleep 2; echo "$order (asked camera 300 -200, zoom 0.8) -> $(dw msg state | grep -E '^(camera|zoom)' | tr '\n' ' ')"
done
kill $(cat $T/dw.pid)
