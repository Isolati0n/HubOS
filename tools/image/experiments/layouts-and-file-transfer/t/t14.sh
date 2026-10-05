#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test 14: DEBUG build, same run, no restart: suspend a live window, then relaunch it.  Does the debug assertion fire?
. $LFT/t/dw.sh
export PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1)
cd /tmp
rm -rf /tmp/dwx/driftwm /tmp/dwx/wayland-[2-9]*
echo "binary: $DW"
start_dw $T/cfg-default.toml
foot --app-id=term-a --title=First sleep 1500 >/dev/null 2>&1 &
sleep 3
dw msg suspend term-a; sleep 2
dw msg state | sed -n 4,5p
dw msg relaunch term-a; sleep 4
dw msg state | sed -n 4,5p
echo "alive: $(ps -p $(cat $T/dw.pid) -o pid= | tr -d ' ')"
clean_dw | grep -A1 panicked | cut -c1-200
kill $(cat $T/dw.pid) 2>/dev/null
