#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 9: window rules (static layout): app_id + title glob -> position/size when the window opens; and hot reload.
. $LFT/t/dw.sh
export PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1)
cd /tmp
rm -rf /tmp/dwx/driftwm /tmp/dwx/wayland-[2-9]*
cat > $T/cfg-rules.toml <<'EOT'
[[window_rules]]
app_id = "viewer"
title = "ai-1 - *"
position = [-1500, 400]
size = [800, 600]

[[window_rules]]
app_id = "viewer"
title = "nas - *"
position = [1500, 400]
size = [800, 600]
EOT
start_dw $T/cfg-rules.toml
foot --app-id=viewer --title="ai-1 - viewer" sleep 1500 >/dev/null 2>&1 &
foot --app-id=viewer --title="nas - viewer" sleep 1500 >/dev/null 2>&1 &
foot --app-id=viewer --title="other - viewer" sleep 1500 >/dev/null 2>&1 &
sleep 4
dw msg --json state | jq -c '.. | objects | select(.app_id=="viewer") | {id,title,position,size}'
echo "--- change the rule file while running (hot reload): move ai-1 rule to [-1000,0]"
sed -i 's/position = \[-1500, 400\]/position = [-1000, 0]/' $T/cfg-rules.toml
sleep 3
clean_dw | grep -iE "reload|config" | tail -3 | cut -c1-200
foot --app-id=viewer --title="ai-1 - second" sleep 1500 >/dev/null 2>&1 &
sleep 3
dw msg --json state | jq -c '.. | objects | select(.app_id=="viewer") | {id,title,position,size}'
kill $(cat $T/dw.pid)
