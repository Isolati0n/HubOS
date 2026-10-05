#!/bin/bash
# Shared helpers for run.sh and the stack, client and scene files. Sourced, never run.
# Everything outside the repository goes under $BENCH_TMP (default /tmp/bench). Nothing is installed.

log()  { echo "[$(date +%T.%3N)] $*" >&2; }
die()  { log "FATAL: $*"; exit 1; }
now_ns() { date +%s%N; }

# --- run directory, background processes ----------------------------------------------------------------------------
RD=${RD:-$BENCH_OUT/rd.$$}          # scratch for this run: logs, pids, screenshots
mkdir -p "$RD/logs" "$RD/pids"
clean_dir() { case "$1" in "$BENCH_TMP"/*) rm -rf -- "$1";; *) die "refusing to delete $1";; esac; }

# start_bg NAME CMD...  -> runs CMD detached, own session; its pid is in LAST_PID and $RD/pids/NAME
start_bg() {
  local name=$1; shift
  if declare -F "$1" >/dev/null; then   # a shell function (hub_env, node_env): run it through a bash that has it exported
    setsid nohup bash -c '"$@"' bash "$@" > "$RD/logs/$name.log" 2>&1 < /dev/null &
  else
    setsid nohup "$@" > "$RD/logs/$name.log" 2>&1 < /dev/null &
  fi
  LAST_PID=$!
  echo $LAST_PID > "$RD/pids/$name"
}
kill_tree() {
  local p=$1 c
  for c in $(pgrep -P "$p" 2>/dev/null); do kill_tree "$c"; done
  kill "$p" 2>/dev/null || true
}
stop_all() {   # polite kill of everything start_bg started, then force
  local f p
  pkill -x wl-copy 2>/dev/null
  for f in "$RD"/pids/*; do [ -e "$f" ] || continue; p=$(cat "$f"); kill_tree "$p"; done
  sleep 0.5
  for f in "$RD"/pids/*; do [ -e "$f" ] || continue; p=$(cat "$f"); kill -9 "$p" 2>/dev/null || true; rm -f "$f"; done
}
signal_tree() {   # signal_tree SIGNAL PID  -> the process and all its children (start_bg may have put a shell wrapper in front of the program)
  local c
  for c in $(pgrep -P "$2" 2>/dev/null); do signal_tree "$1" "$c"; done
  kill -"$1" "$2" 2>/dev/null || true
}
leaf_pid() {      # leaf_pid PID -> the program at the end of a chain of single children (the real viewer, for sway's [pid=] criteria)
  local p=$1 c
  while c=$(pgrep -P "$p" 2>/dev/null | head -1); [ -n "$c" ]; do p=$c; done
  echo "$p"
}
wait_for() {   # wait_for SECONDS COMMAND...   (polls every 0.1 s)
  local t=$1; shift; local end=$(( $(date +%s%N) + t * 1000000000 ))
  while ! "$@" >/dev/null 2>&1; do [ "$(date +%s%N)" -gt "$end" ] && return 1; sleep 0.1; done
}
wait_socket() {  # wait_socket DIR -> prints the name of the first wayland socket in DIR
  local d=$1 s end=$(( $(date +%s) + 20 ))
  while [ "$(date +%s)" -lt "$end" ]; do
    for s in "$d"/wayland-* "$d"/wl-*; do
      [ -S "$s" ] && { basename "$s"; return 0; }
    done
    sleep 0.1
  done
  return 1
}

# --- node side: helpers the stack files use ---------------------------------------------------------------------------
# node_run CMD...   runs CMD as a client of the node's compositor
node_run() {
  start_bg "app-$(basename "$1")" env XDG_RUNTIME_DIR="$NODE_RT" WAYLAND_DISPLAY="$NODE_WL" "$@"
  NODE_APP_PIDS+=("$LAST_PID")
}
node_env() { env XDG_RUNTIME_DIR="$NODE_RT" WAYLAND_DISPLAY="$NODE_WL" "$@"; }
node_screenshot_wlr() { node_env grim "$1"; }   # wlr-screencopy capture of the node's own output

# weston_capture FILE.png   Weston's own screenshot client (needs weston started with --debug)
weston_capture() {
  local d=$RD/shots; mkdir -p "$d"; clean_dir "$d"; mkdir -p "$d"
  ( cd "$d" && node_env timeout 20 weston-screenshooter >/dev/null 2>&1 )
  local f; f=$(ls -t "$d"/*.png 2>/dev/null | head -1); [ -n "$f" ] && mv -f "$f" "$1"
}

# foot_cmd -> array FOOT: a white terminal window with a fixed font (the same everywhere, so text looks the same on all stacks)
foot_cmd() { FOOT=(foot -o 'font=DejaVu Sans Mono:size=12' -o colors.background=b0c8f0 -o colors.foreground=000000 -o cursor.blink=no -o pad=6x6 -o bell.urgent=no "$@"); }

# --- hub side -----------------------------------------------------------------------------------------------------------
# hub_start W H    a second headless sway: the "hub". Viewers are its windows, fullscreen, so one hub pixel = one node pixel.
hub_start() {
  HUB_DISPLAY=
  local w=$1 h=$2
  HUB_RT=$BENCH_RUN/hub; clean_dir "$HUB_RT"; mkdir -p -m700 "$HUB_RT"
  cat > "$HUB_RT/sway.conf" <<CONF
default_border none
xwayland enable
focus_follows_mouse no
output HEADLESS-1 resolution ${w}x${h}
CONF
  [ "${HUB_FULLSCREEN:-1}" = 1 ] && printf 'for_window [app_id=".*"] fullscreen enable\nfor_window [class=".*"] fullscreen enable\n' >> "$HUB_RT/sway.conf"
  start_bg hub-sway env XDG_RUNTIME_DIR="$HUB_RT" sway -c "$HUB_RT/sway.conf"
  HUB_COMP_PID=$LAST_PID
  HUB_WL=$(wait_socket "$HUB_RT") || die "hub compositor did not start: $(tail -3 "$RD/logs/hub-sway.log")"
  wait_for 10 sh -c "ls $HUB_RT/sway-ipc.*.sock" || die "no sway socket"
  HUB_SWAYSOCK=$(ls "$HUB_RT"/sway-ipc.*.sock | head -1)
  hub_env swaymsg exec "sh -c 'echo \$DISPLAY > $HUB_RT/display'" >/dev/null
  wait_for 5 test -s "$HUB_RT/display"; HUB_DISPLAY=$(cat "$HUB_RT/display" 2>/dev/null)
  export HUB_RT HUB_WL HUB_SWAYSOCK HUB_DISPLAY
}
hub_env() { env XDG_RUNTIME_DIR="$HUB_RT" WAYLAND_DISPLAY="$HUB_WL" SWAYSOCK="$HUB_SWAYSOCK" DISPLAY="$HUB_DISPLAY" "$@"; }
hub_screenshot() { hub_env grim "$1"; }
hub_tree() { hub_env swaymsg -t get_tree; }
export -f hub_env node_env

# --- byte counter (tcpmeter) -------------------------------------------------------------------------------------------
meter_start() {   # meter_start LISTEN TARGET
  METER_STATS=$RD/meter.txt
  start_bg meter python3 "$BENCH_DIR/metrics/tcpmeter.py" "$1" "$2" "$METER_STATS"; METER_PID=$LAST_PID
  wait_for 5 test -s "$METER_STATS"
}
meter_read() { cat "$METER_STATS"; }          # s2c c2s accepted open last_accept_ns first_server_byte_ns

# --- results ---------------------------------------------------------------------------------------------------------------
res() { echo "$1=$2" >> "$RES_TMP"; }
res_kv() { local l; while IFS= read -r l; do [ -n "$l" ] && res "${l%%=*}" "${l#*=}"; done; }   # key=value lines on stdin
load_now() { cut -d' ' -f1-3 /proc/loadavg; }

# --- CPU and memory ---------------------------------------------------------------------------------------------------
snap_groups() {   # snap_groups FILE   -> groups: node_app, node_comp, node_srv, hub_viewer, hub_comp
  local apps; apps=$(IFS=,; echo "${NODE_APP_PIDS[*]}")
  local g=()
  [ -n "$apps" ] && g+=("node_app=$apps")
  g+=("node_srv=${NODE_SRV_PID:-0}" "node_comp=${NODE_COMP_PID:-0}")
  g+=("hub_viewer=${CLIENT_PID:-0}" "hub_comp=${HUB_COMP_PID:-0}")
  python3 "$BENCH_DIR/metrics/procstat.py" snap "$1" "${g[@]}"
}
res_diff() {      # res_diff PREFIX SNAP1 SNAP2   -> PREFIX_<group>_cpu / _rss_mb / _hwm_mb
  local pre=$1 l c r m n p
  while read -r l c r m n p; do
    res "${pre}_${l}_cpu" "$c"; res "${pre}_${l}_rss_mb" "$r"; res "${pre}_${l}_hwm_mb" "$m"; res "${pre}_${l}_pss_mb" "$p"
  done < <(python3 "$BENCH_DIR/metrics/procstat.py" diff "$2" "$3")
}
# measure_window PREFIX SECONDS   bytes on the socket plus CPU and memory over the window
measure_window() {
  local pre=$1 secs=$2 a b
  res "${pre}_load_before" "$(load_now)"
  snap_groups "$RD/s1.json"; a=($(meter_read)); local t0; t0=$(now_ns)
  sleep "$secs"
  b=($(meter_read)); snap_groups "$RD/s2.json"; local t1; t1=$(now_ns)
  local dt_ms=$(( (t1 - t0) / 1000000 ))
  res "${pre}_secs" "$(python3 -c "print(round($dt_ms/1000,2))")"
  res "${pre}_s2c_bytes" "$(( b[0] - a[0] ))"
  res "${pre}_c2s_bytes" "$(( b[1] - a[1] ))"
  res "${pre}_s2c_kbit_s" "$(python3 -c "print(round(($((b[0]-a[0]))*8/1000)/($dt_ms/1000),1))")"
  res_diff "$pre" "$RD/s1.json" "$RD/s2.json"
}

# --- probe: wait until a hub pixel shows the node's colour ---------------------------------------------------------------
# probe_start X,Y    starts pixelwatch on the hub (log in $RD/probe.log)
probe_start() { start_bg probe hub_env pixelwatch -t 600 "$@"; PROBE_PID=$LAST_PID; }
# wait_pixel T0_NS EXPECT TOL TIMEOUT_S   prints ms from T0 to the first hub frame with that pixel value, or "none"
wait_pixel() {
  local t0=$1 e=$2 tol=$3 to=$4 end=$(( $(date +%s) + $4 )) r
  PROBE_LOG=$RD/logs/probe.log
  while [ "$(date +%s)" -lt "$end" ]; do
    r=$(python3 "$BENCH_DIR/metrics/analyze.py" ttff "$PROBE_LOG" "$t0" "$e" "$tol")
    [ "$r" != none ] && { echo "$r"; return 0; }
    sleep 0.25
  done
  echo none; return 1
}
