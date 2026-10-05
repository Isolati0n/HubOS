#!/bin/bash
# What do stock viewers (VIEWER=tiger: TigerVNC 1.13.1 through Xwayland; VIEWER=wlvncc) do when their window is
#   full size / made smaller / hidden (sway scratchpad) / shown again?
# Needs: node.sh's programs, plus xtigervncviewer, Xwayland, xkbcomp unpacked into $BENCH_TMP/a/root, and the
# counting relay tools/bench/remote-display/metrics/tcpmeter.py from branch bench-remote-display-clean
# (set TCPMETER=path). Everything runs in a private mount namespace (unshare -m) that shows the unpacked
# /usr/bin read-only; nothing is installed.   usage: BENCH_TMP=/tmp/bench TCPMETER=... ./viewer-test.sh
set -u
ulimit -c 0
cd "$(dirname "$0")"
export BENCH_TMP=${BENCH_TMP:-/tmp/bench}
TCPMETER=${TCPMETER:?set TCPMETER}
if [ "${1:-}" != inner ]; then
  exec unshare -m sh -c 'mount -t overlay overlay -o lowerdir='"$BENCH_TMP"'/a/root/usr/bin:/usr/bin /usr/bin && exec bash "$0" inner' "$0"
fi
ROOT=$BENCH_TMP/a/root; PREFIX=$BENCH_TMP/prefix
export PATH=$PREFIX/bin:$ROOT/usr/bin:$PATH
export LD_LIBRARY_PATH=$ROOT/usr/lib/x86_64-linux-gnu:$ROOT/lib/x86_64-linux-gnu:$PREFIX/lib:$PREFIX/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}
export XDG_DATA_DIRS=$ROOT/usr/share:/usr/share
export WLR_RENDERER=pixman WLR_BACKENDS=headless WLR_LIBINPUT_NO_DEVICES=1
D=$BENCH_TMP/lod; HRT=$D/hubrt; STATS=$D/tm.stats
mkdir -p -m700 "$HRT"; rm -f "$STATS"*
PIDS_EXTRA=()

./node.sh start clock 1280 720 5901 -Ldebug > $D/node.pids || exit 1
python3 "$TCPMETER" 5902 5901 "$STATS" > $D/tm.log 2>&1 & PIDS_EXTRA+=($!)
cat > "$HRT/sway.conf" <<CONF
default_border none
output HEADLESS-1 resolution 1280x720
CONF
XDG_RUNTIME_DIR=$HRT sway -c "$HRT/sway.conf" > $D/hubsway.log 2>&1 & PIDS_EXTRA+=($!)
for _ in $(seq 100); do ls $HRT/sway-ipc.*.sock >/dev/null 2>&1 && break; sleep 0.1; done
export SWAYSOCK=$(ls $HRT/sway-ipc.*.sock | head -1) XDG_RUNTIME_DIR=$HRT
sleep 1
VIEWER=${VIEWER:-tiger}     # tiger = xtigervncviewer (Xwayland), wlvncc = Wayland-native viewer
if [ "$VIEWER" = wlvncc ]; then CRIT='[app_id=".*"]'; VCMD="wlvncc 127.0.0.1 5902"
else CRIT='[class=".*"]'; VCMD="xtigervncviewer -SecurityTypes None -AutoSelect=0 127.0.0.1::5902"; fi
swaymsg exec "$VCMD" > /dev/null
sleep 6

sample() {  # sample LABEL SECONDS : server->client bytes per second over the period
  local a b t0 t1
  a=$(cut -d' ' -f1 "$STATS"); t0=$(date +%s.%N); sleep "$2"; b=$(cut -d' ' -f1 "$STATS"); t1=$(date +%s.%N)
  python3 -c "print('%-34s %8.1f kB/s server->client' % ('$1', ($b-$a)/1000.0/($t1-$t0)))"
}
tree() { swaymsg -t get_tree | python3 -c "
import json,sys
t=json.load(sys.stdin)
def walk(n):
    if n.get('window') or (n.get('app_id') or '') != '':
        print('  window:', n.get('name'), 'rect', n['rect']['width'],'x',n['rect']['height'], 'visible', n.get('visible'), 'floating', n.get('type'))
    for c in n.get('nodes',[])+n.get('floating_nodes',[]): walk(c)
walk(t)"; }
echo "# viewer test ($VIEWER); wayvnc log lines about the viewer:"
grep -a 'set encodings\|Choosing\|resize' $D/wayvnc.log | cut -c1-200
echo "# window as opened:"; tree
sample "full size, visible" 6
swaymsg "$CRIT floating enable, resize set 640 360" > /dev/null; sleep 3
echo "# after floating + resize to 640x360:"; tree
grep -a 'resize' $D/wayvnc.log | cut -c1-200
sample "window 640x360 (after 3 s settle)" 6
swaymsg "$CRIT move scratchpad" > /dev/null; sleep 2
echo "# after move to scratchpad (hidden):"; tree
sample "hidden (scratchpad)" 8
swaymsg 'scratchpad show' > /dev/null; sleep 2
echo "# after scratchpad show:"; tree
sample "shown again" 6
echo "# client messages seen in the first 4 KB the viewer sent (type:count):"
python3 - "$STATS.c2s" <<'EOF'
import sys, struct, collections
b = open(sys.argv[1], 'rb').read()
if b[:3] == b'RFB': b = b[12:]
# skip security choice (1 byte) and ClientInit (1 byte)
b = b[2:]
p = 0; names = {0: 'SetPixelFormat', 2: 'SetEncodings', 3: 'FBUpdateRequest', 4: 'KeyEvent', 5: 'PointerEvent', 6: 'ClientCutText', 150: 'EnableContinuousUpdates', 248: 'Fence', 251: 'SetDesktopSize'}
cnt = collections.Counter(); order = []
while len(b) - p >= 10:   # (a message cut off at the 4 KB end is ignored)
    t = b[p]
    if t == 0: n = 20
    elif t == 2: n = 4 + 4 * struct.unpack_from('>H', b, p + 2)[0]
    elif t == 3: n = 10
    elif t == 4: n = 8
    elif t == 5: n = 6
    elif t == 6: n = 8 + abs(struct.unpack_from('>i', b, p + 4)[0])
    elif t == 150: n = 10
    elif t == 251: n = 8 + 16 * b[p + 6]
    elif t == 248: n = 9 + b[p + 8]
    else: break
    if p + n > len(b): break
    nm = names.get(t, t)
    if t == 150: nm = 'EnableContinuousUpdates(enable=%d,region=%s)' % (b[p + 1], struct.unpack_from('>HHHH', b, p + 2))
    if t == 3: nm = 'FBUpdateRequest(incremental=%d)' % b[p + 1]
    cnt[names.get(t, t)] += 1
    if len(order) < 12: order.append(nm)
    p += n
print('  ', dict(cnt), 'first messages:', order)
EOF
swaymsg exit > /dev/null 2>&1
for p in "${PIDS_EXTRA[@]}"; do kill "$p" 2>/dev/null; done
./node.sh stop
