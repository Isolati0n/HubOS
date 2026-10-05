#!/bin/bash
# The campaign behind docs/proposals/remote-display-benchmarks.md. Resumable: re-run it and finished cells are skipped.
# Roughly 4 to 6 hours on 4 cores. Results land in results/raw/. Then:  ./summarize.py   and   ./versions.sh > results/versions.txt
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$HERE" || exit 1
export KEEP_LOGS=${KEEP_LOGS:-}
# A. the main grid at 1080p, three runs each
RUNS=3 RESES=1080p SCENES="idle scroll term drag video latency fidelity" ./run-all.sh
# B. behaviour tests at 1080p (what happens, not how fast): once or twice
RUNS=1 RESES=1080p SCENES="quiet smallsize clipboard resize" ./run-all.sh
RUNS=2 RESES=1080p SCENES="reconnect" ./run-all.sh
# C. 4K: text fidelity, idle, scrolling and drag, two runs
RUNS=2 RESES=4k SCENES="fidelity idle scroll drag latency" ./run-all.sh
# D. the kiosk compositor (cage) can only do 720p
RUNS=3 RESES=720p STACKS="wayvnc-cage" SCENES="idle scroll term drag video latency fidelity" ./run-all.sh
RUNS=1 RESES=720p STACKS="wayvnc-cage" SCENES="quiet smallsize clipboard reconnect" ./run-all.sh
# E. scale: 20 simultaneous idle sessions into one hub, for each VNC viewer
for c in wlvncc tigervnc remote-viewer; do ./scale.sh wayvnc-sway $c 20 1080p 1; done
