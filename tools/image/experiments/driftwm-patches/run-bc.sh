#!/bin/bash
# run-bc.sh UNPATCHED_BIN PATCHED_BIN : the minimal reproductions of docs/proposals/bulletproof-compositor.md (BC-1, BC-2, BC-3,
# BC-6, BC-12, BC-13) against an unpatched and a patched driftwm, one result line each (verify.sh of that folder does the
# work). BC-11 is a pure-logic overflow: its reproduction is the unit tests in image/patches/driftwm/0018-d6-*.patch
# (run with `cargo test --release bc11`), not a live compositor.
# Needs HS_WORK with A/root (the build root plus Xvfb, xkbcomp, foot and the Mesa software drivers unpacked with dpkg -x;
# nothing is installed) and root (Xvfb runs in a private mount namespace). Runs everything under /tmp; ulimit -c 0.
ulimit -c 0
HERE=$(cd "$(dirname "$0")" && pwd)
SRC=$HERE/../../../../docs/proposals/bulletproof-compositor/scripts
: "${HS_WORK:?set HS_WORK}"
export BC_WORK=$HS_WORK/bc
mkdir -p "$BC_WORK/bin" "$BC_WORK/rr"
cp "$SRC"/*.sh "$SRC"/*.py "$BC_WORK/"
[ -e "$BC_WORK/A" ] || ln -s "$HS_WORK/A" "$BC_WORK/A"
ln -sf "$(readlink -f "$1")" "$BC_WORK/bin/unpatched"
ln -sf "$(readlink -f "$2")" "$BC_WORK/bin/patched"
bash "$BC_WORK/xvfb.sh" :93 1920x1080x24 > "$BC_WORK/xvfb.log" 2>&1 &
XPID=$!
for i in $(seq 1 50); do [ -S /tmp/.X11-unix/X93 ] && break; sleep 0.2; done
for which in unpatched patched; do
  echo "######## $which"
  bash "$BC_WORK/verify.sh" "$which" 2>&1
done
kill $XPID 2>/dev/null; wait $XPID 2>/dev/null
exit 0
