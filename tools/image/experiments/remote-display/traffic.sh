#!/bin/bash
# Bytes on the display socket for an idle desktop, scrolling text and a video.
# Needs a node compositor (any), a display server on it, tcpmeter.py between the viewer and the server
# (writing STATS), and a viewer connected through the meter.
# Usage: traffic.sh NODE_WAYLAND_DISPLAY STATS_FILE SECONDS   (XDG_RUNTIME_DIR must be set)
# The test windows use app-id "load". Other windows on the node are left alone.
ND=$1; STATS=$2; T=${3:-20}
stopload() { for n in foot mpv; do for p in $(pgrep -x $n); do
    case "$(tr '\0' ' ' </proc/$p/cmdline)" in *load*) kill $p;; esac; done; done; }
measure() {  # label
  local a b
  a=($(cat "$STATS")); sleep "$T"; b=($(cat "$STATS"))
  echo "$1: server->client $(( b[0]-a[0] )) bytes in ${T}s = $(( (b[0]-a[0])*8/T/1000 )) kbit/s; client->server $(( b[1]-a[1] )) bytes"
}
export WAYLAND_DISPLAY=$ND
stopload; sleep 2
measure "idle (no window changes)"
foot --app-id=load sh -c 'while :; do cat /proc/uptime; done' >/dev/null 2>&1 &
sleep 3; measure "scrolling text (foot printing the uptime line as fast as it can, so every frame differs; load)"
stopload; sleep 2
mpv --no-config --vo=wlshm --no-audio --force-window=yes --geometry=640x480 --title=load --wayland-app-id=load \
  'av://lavfi:testsrc2=size=640x480:rate=30' >/dev/null 2>&1 &
sleep 4; measure "video (mpv, moving test pattern 640x480, 30 fps, load)"
stopload
