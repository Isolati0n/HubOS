#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Runs every test in order with the RELEASE driftwm and writes one log.  Needs: up.sh and vp-up.sh already run.
T=$LFT/t
W=$LFT
export DWBIN=$W/target-rel/release/driftwm
stopall() { for p in dw cli-a cli-b; do [ -f $T/$p.pid ] && kill $(cat $T/$p.pid) 2>/dev/null; done; sleep 1; rm -rf /tmp/dwx/driftwm /tmp/dwx/wayland-[2-9]*; }
sec() { echo; echo "################################################################ $1"; }
stopall
sec "L1-L3: save, kill -9, restart, relaunch, dismiss (all four [session] flags on)"; bash $T/rel123.sh
stopall; sec "L4: default flags (all off) + --session-file; explicit suspend; kill -9; restart; relaunch"; bash $T/t4.sh 2>&1
stopall; sec "L5: two windows with one app_id"; bash $T/t7.sh 2>&1
stopall; sec "L6: robustness of the session file"; bash $T/t8.sh 2>&1
stopall; sec "L7: window rules (static layout) and hot reload"; bash $T/t9.sh 2>&1
stopall; sec "L8: stale files after kill -9, restart without cleanup"; bash $T/t11.sh 2>&1
stopall; sec "L9: camera and zoom set back to back"; bash $T/t12.sh 2>&1
stopall; sec "L10: suspend then relaunch in the same run: DEBUG build, then RELEASE build"; DWBIN=$W/target/debug/driftwm bash $T/t14.sh 2>&1; bash $T/t14.sh 2>&1
stopall; sec "D1: implicit grab (plain client A holds the pointer)"; bash $T/run56.sh plain 2>&1 | sed -n '/########/,$p'
stopall; sec "D2: a real Wayland drag from A (source) to B (target)"; bash $T/run56.sh src 2>&1 | sed -n '/########/,$p'
stopall; sec "D3: overlay mapped DURING the drag"; kill $(cat $T/cli-a.pid) $(cat $T/cli-b.pid) 2>/dev/null; bash $T/t10.sh 2>&1
stopall; sec "D4: overlay mapped BEFORE the press (armed)"; kill $(cat $T/cli-a.pid) $(cat $T/cli-b.pid) 2>/dev/null; bash $T/t13.sh 2>&1
stopall
sec "F1: file clipboard as text/uri-list"; bash $T/tclip.sh 2>&1
sec "F2: one-time signed URLs (prototype)"; bash $T/tft.sh 2>&1
sec "F3: rsync daemon and sftp-server"; bash $T/trs.sh 2>&1
sec "F4: adapter conformance test"; bash $T/tconf.sh 2>&1
sec "F5: end to end, two fake nodes"; bash $T/te2e.sh 2>&1
