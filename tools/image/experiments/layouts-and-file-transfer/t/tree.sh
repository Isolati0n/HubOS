#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
. $LFT/t/dw.sh
export SWAYSOCK=$(ls /tmp/dwx/sway-ipc.*.sock | head -1)
swaymsg -t get_tree | jq -c '.. | objects | select(.pid? and .type=="con") | {pid, name, app_id, w:.rect.width, h:.rect.height}'
echo "driftwm pids:"; pgrep -a driftwm | cut -c1-100
