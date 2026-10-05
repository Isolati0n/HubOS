#!/bin/bash
# The only node-side way to make wayvnc stop capturing is to have no client at all (wayvnc src/main.c, client_destroy:
# "Stopping screen capture" when nr_clients == 0). This measures CPU of wayvnc / sway / foot for 6 s WITH a client that
# asks for updates normally, then for 6 s after that client has disconnected.   BENCH_TMP=/tmp/bench ./disconnect.sh
set -u
ulimit -c 0
cd "$(dirname "$0")"
export BENCH_TMP=${BENCH_TMP:-/tmp/bench}
OUT=results; mkdir -p $OUT
PORT=5901
./node.sh start scroll 1280 720 $PORT > $OUT/.pids.$$ || exit 1
sleep 3
args=""; while read -r n p; do args="$args $n=$p"; done < $OUT/.pids.$$
cpu() {
python3 - "$@" <<'PY'
import os, sys, time
pids = dict(a.split('=') for a in sys.argv[1:])
clk = os.sysconf('SC_CLK_TCK')
def t(p):
    s = open('/proc/%s/stat' % p).read()
    r = s[s.rindex(')') + 2:].split()
    return int(r[11]) + int(r[12])
a = {k: t(v) for k, v in pids.items()}
t0 = time.time(); time.sleep(6); d = time.time() - t0
print(' '.join('%s=%.1f' % (k, 100.0 * (t(v) - a[k]) / clk / d) for k, v in pids.items()))
PY
}
{
  echo "scene=scroll size=1280x720 wayvnc_args=''"
  python3 rfbprobe.py --port $PORT --phases 'normal:60' > /dev/null 2>&1 &
  PROBE=$!
  sleep 4
  echo "cpu% over 6 s WITH a client (normal requests): $(cpu $args)"
  kill $PROBE; wait $PROBE 2>/dev/null
  sleep 2
  echo "cpu% over 6 s with NO client connected:        $(cpu $args)"
} > $OUT/disconnect-scroll.txt 2>&1
./node.sh stop
rm -f $OUT/.pids.$$
cat $OUT/disconnect-scroll.txt
