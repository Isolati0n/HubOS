#!/bin/bash
# run-all.sh UNPATCHED_BIN PATCHED_BIN [UNPATCHED_HOOKS_BIN PATCHED_HOOKS_BIN [UNPATCHED_BADSHADER_BIN PATCHED_BADSHADER_BIN]]
# Runs the reproductions of docs/proposals/hub-stability/ (repros.py) against the unpatched and the patched compositor and
# prints the VERDICT lines. The optional third and fourth arguments are builds that also contain the test hook patch
# image/patches/driftwm/not-applied/0002-p2-test-hooks.patch (unpatched+hooks, patched+hooks) for the "panic while the log
# pipe is full" case. The fifth and sixth are builds whose built-in default shader (src/shaders/dot_grid.glsl) was replaced by garbage (patch P9).
# Needs HS_WORK (see repros.py), root (Xvfb runs in a private mount namespace, xvfb-run-ns.sh). Nothing is installed.
ulimit -c 0
HERE=$(cd "$(dirname "$0")" && pwd)
: "${HS_WORK:?set HS_WORK}"
export HS_WORK
U=$1; P=$2; UT=$3; PT=$4; UB=$5; PB=$6
bash "$HERE/xvfb-run-ns.sh" :81 1280x800x24 > /tmp/dpr-xvfb.log 2>&1 &
XPID=$!
for i in $(seq 1 50); do [ -S /tmp/.X11-unix/X81 ] && break; sleep 0.2; done
run() { timeout 240 python3 "$HERE/repros.py" "$@" 2>&1 | sed 's/^/    /'; }
for c in shm config pipe startup reload session; do
  for pair in "unpatched:$U" "patched:$P"; do
    label=${pair%%:*}; bin=${pair#*:}
    echo "== $c / $label"
    run "$c" "$bin" "$label"
  done
done
if [ -n "$UT" ] && [ -n "$PT" ]; then
  for pair in "unpatched+hooks:$UT" "patched+hooks:$PT"; do
    label=${pair%%:*}; bin=${pair#*:}
    echo "== pipepanic / $label"
    run pipepanic "$bin" "$label"
  done
fi
if [ -n "$UB" ] && [ -n "$PB" ]; then
  for pair in "unpatched+badshader:$UB" "patched+badshader:$PB"; do
    label=${pair%%:*}; bin=${pair#*:}
    echo "== badshader / $label"
    run badshader "$bin" "$label"
  done
fi
kill $XPID 2>/dev/null; wait $XPID 2>/dev/null
exit 0
