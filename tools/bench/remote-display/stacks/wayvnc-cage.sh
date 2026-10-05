# STACK: headless cage (a one-application "kiosk" wlroots compositor) + wayvnc + neatvnc. See wayvnc-sway.sh for the contract.
STACK_DESC="headless cage 0.1.5 (kiosk) + wayvnc v0.10.2 + neatvnc v1.0.3, pixman renderer"
STACK_PROTO=vnc
STACK_CAN_RESIZE=0
STACK_ONLY_RES=720p   # TESTED: cage 0.1.5 crashes (assertion in wlr_scene_output_layout_add_output) when the headless output's mode is changed, so it stays at 1280x720

stack_start() {
  local i=$1 w=$2 h=$3; shift 3
  NODE_RT=$BENCH_RUN/node$i; clean_dir "$NODE_RT"; mkdir -p -m700 "$NODE_RT"
  NODE_PORT=$((5900 + i))
  # cage runs exactly one program (its only window) and quits when it quits
  start_bg node$i-cage env XDG_RUNTIME_DIR="$NODE_RT" cage -- "$@"
  NODE_COMP_PID=$LAST_PID
  NODE_APP_LOG=$RD/logs/node$i-cage.log   # cage starts the program itself, so its output lands in cage's log
  export NODE_APP_LOG
  NODE_WL=$(wait_socket "$NODE_RT") || die "node cage did not start: $(tail -3 "$RD/logs/node$i-cage.log")"
  wait_for 5 pgrep -P "$NODE_COMP_PID"; local c; for c in $(pgrep -P "$NODE_COMP_PID"); do [ "$(cat /proc/$c/comm)" != Xwayland ] && NODE_APP_PIDS+=("$c"); done   # the program cage started, counted as "app" like on the other stacks
  start_bg node$i-wayvnc env XDG_RUNTIME_DIR="$NODE_RT" WAYLAND_DISPLAY="$NODE_WL" wayvnc -n "node$i" -o HEADLESS-1 127.0.0.1 "$NODE_PORT"
  NODE_SRV_PID=$LAST_PID
  wait_for 10 python3 -c "import socket;socket.create_connection(('127.0.0.1',$NODE_PORT)).close()" || die "wayvnc did not listen: $(tail -3 "$RD/logs/node$i-wayvnc.log")"
}
stack_capture() { node_screenshot_wlr "$1"; }
stack_versions() { echo "cage $(cage -v 2>&1 | head -1); $(wayvnc --version 2>&1 | head -2 | tr '\n' ' ')"; }
