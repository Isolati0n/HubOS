#!/bin/bash
# Starts / stops ONE fake node for the level-of-detail experiments: headless sway + a foot terminal that draws a
# moving scene + wayvnc (pinned build). Nothing is installed; programs come from $BENCH_TMP, which the harness
# setup (tools/bench/remote-display/setup.sh on branch bench-remote-display-clean) filled.
#
#   node.sh start SCENE [WIDTH HEIGHT PORT [WAYVNC_EXTRA_ARGS...]]    SCENE = clock | scroll | idle
#   node.sh stop
#
# It writes $BENCH_TMP/lod/pids (one "name pid" line each) and logs. It prints the PIDs. It starts no
# process by name-matching and stops only the PIDs it wrote.
set -u
ulimit -c 0
BENCH_TMP=${BENCH_TMP:-/tmp/bench}
ROOT=$BENCH_TMP/a/root; PREFIX=$BENCH_TMP/prefix
export PATH=$PREFIX/bin:$ROOT/usr/bin:$PATH
export LD_LIBRARY_PATH=$ROOT/usr/lib/x86_64-linux-gnu:$ROOT/lib/x86_64-linux-gnu:$PREFIX/lib:$PREFIX/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}
export XDG_DATA_DIRS=$ROOT/usr/share:$PREFIX/share:/usr/share
export WLR_RENDERER=pixman WLR_BACKENDS=headless WLR_LIBINPUT_NO_DEVICES=1
D=$BENCH_TMP/lod; RT=$D/rt; PIDS=$D/pids
# fonts: the machine's own /usr/share/fonts (DejaVu) plus the unpacked ones, with a private cache folder
mkdir -p "$D"
printf '<?xml version="1.0"?><!DOCTYPE fontconfig SYSTEM "fonts.dtd"><fontconfig><dir>/usr/share/fonts</dir><dir>%s/usr/share/fonts</dir><cachedir>%s/fontcache</cachedir></fontconfig>\n' "$ROOT" "$D" > "$D/fonts.conf"
export FONTCONFIG_FILE=$D/fonts.conf

case ${1:-} in
start)
  scene=${2:-clock}; W=${3:-1280}; H=${4:-720}; PORT=${5:-5901}; shift $(( $# < 5 ? $# : 5 ))
  [ -s "$PIDS" ] && { echo "already started; run stop first" >&2; exit 1; }
  mkdir -p -m700 "$D" "$RT"; : > "$PIDS"
  cat > "$RT/sway.conf" <<CONF
default_border none
focus_follows_mouse no
output HEADLESS-1 resolution ${W}x${H}
for_window [app_id=".*"] fullscreen enable
CONF
  case $scene in
    clock)  CMD='i=0; while :; do i=$((i+1)); printf "\r clock %d   " $i; sleep 0.03; done' ;;   # a small changing patch, about 30 changes/s
    scroll) CMD='seq 1 100000000' ;;                                                              # the whole window scrolls as fast as foot can draw
    idle)   CMD='echo idle; exec sleep 100000' ;;
    *) echo "unknown scene" >&2; exit 1 ;;
  esac
  XDG_RUNTIME_DIR=$RT setsid nohup sway -c "$RT/sway.conf" > "$D/sway.log" 2>&1 < /dev/null & echo "sway $!" >> "$PIDS"
  for _ in $(seq 100); do ls "$RT"/wayland-* >/dev/null 2>&1 && break; sleep 0.1; done
  WL=$(basename "$(ls "$RT"/wayland-* | grep -v lock | head -1)")
  XDG_RUNTIME_DIR=$RT WAYLAND_DISPLAY=$WL setsid nohup foot -o 'font=DejaVu Sans Mono:size=12' -o colors.background=b0c8f0 -o colors.foreground=000000 -o cursor.blink=no \
      bash -c "$CMD" > "$D/foot.log" 2>&1 < /dev/null & echo "foot $!" >> "$PIDS"
  sleep 1
  XDG_RUNTIME_DIR=$RT WAYLAND_DISPLAY=$WL setsid nohup wayvnc "$@" -n lodnode -o HEADLESS-1 127.0.0.1 "$PORT" > "$D/wayvnc.log" 2>&1 < /dev/null & echo "wayvnc $!" >> "$PIDS"
  for _ in $(seq 100); do python3 -c "import socket;socket.create_connection(('127.0.0.1',$PORT)).close()" 2>/dev/null && break; sleep 0.1; done
  cat "$PIDS"
  ;;
stop)
  [ -s "$PIDS" ] || exit 0
  while read -r n p; do kill "$p" 2>/dev/null; done < "$PIDS"
  sleep 1
  while read -r n p; do kill -9 "$p" 2>/dev/null; done < "$PIDS"
  : > "$PIDS"
  ;;
*) echo "usage: node.sh start SCENE [W H PORT [wayvnc args]] | stop" >&2; exit 1 ;;
esac
