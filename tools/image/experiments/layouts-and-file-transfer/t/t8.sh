#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 8: robustness of the session file: corrupt, newer version, unwritable path, SIGTERM, per-app rule.
. $LFT/t/dw.sh
export PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1)
cd /tmp
SD=$T/state/driftwm; SF=$SD/session.json
fresh() { rm -rf $T/state/* /tmp/dwx/driftwm /tmp/dwx/wayland-[2-9]*; mkdir -p $SD; }
stopdw() { kill $(cat $T/dw.pid) 2>/dev/null; sleep 1; rm -f /tmp/dwx/wayland-[2-9]*; }

echo "##### A. corrupt file"; fresh
echo '{ not json ][' > $SF
start_dw $T/cfg-on.toml --session-file $SF
ls $SD; dw msg state | sed -n 4,5p; clean_dw | grep -iE "warn|quarant" | grep -vE "EGL|xkb|cosmic|xdg_toplevel|bus|xwayland" | cut -c1-200
foot --app-id=term-a --title=First sleep 1500 >/dev/null 2>&1 & sleep 3; sleep 2; ls $SD; stopdw

echo "##### B. file from a NEWER version (version 999)"; fresh
echo '{"version":999,"saved_at":0,"entries":[],"outputs":{}}' > $SF
start_dw $T/cfg-on.toml --session-file $SF
ls $SD; stopdw

echo "##### C. unwritable path (parent is a regular file; stands in for a read-only root)"; fresh
start_dw $T/cfg-on.toml --session-file /dev/null/x/session.json
foot --app-id=term-a --title=First sleep 1500 >/dev/null 2>&1 & sleep 4
dw msg state | sed -n 4,5p
echo "driftwm alive: $(ps -p $(cat $T/dw.pid) -o pid= | tr -d ' ')"
clean_dw | grep -iE "session" | cut -c1-220 | head -5
stopdw

echo "##### D. SIGTERM right after a move: is the last change saved?"; fresh
start_dw $T/cfg-on.toml --session-file $SF
foot --app-id=term-a --title=First sleep 1500 >/dev/null 2>&1 & sleep 3
dw msg move 111 222 --id 0 >/dev/null; kill -TERM $(cat $T/dw.pid); sleep 2
echo "file after SIGTERM:"; ls $SD; [ -f $SF ] && jq -c '.entries[]|{app_id,position}' $SF
rm -f /tmp/dwx/wayland-[2-9]*

echo "##### E. rule restore_windows=false for term-b only"; fresh
cat > $T/cfg-rule.toml <<'EOT'
[session]
restore_windows = true
[[window_rules]]
app_id = "term-b"
restore_windows = false
EOT
start_dw $T/cfg-rule.toml --session-file $SF
foot --app-id=term-a --title=First sleep 1500 >/dev/null 2>&1 &
foot --app-id=term-b --title=Second sleep 1500 >/dev/null 2>&1 & sleep 5
jq -c '.entries[]|{app_id,origin}' $SF
stopdw
