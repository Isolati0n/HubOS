# STACK: headless sway (wlroots, software renderer) + wayvnc + neatvnc. One file = one stack; run.sh sources it.
# Contract (what run.sh needs from a stack file):
#   STACK_DESC, STACK_PROTO (vnc|rdp), STACK_CAN_RESIZE (1 if stack_resize works)
#   stack_start IDX W H APP...   start the node: compositor, the app, the server. Set NODE_RT NODE_WL NODE_PORT
#                                 NODE_COMP_PID NODE_SRV_PID. IDX (1,2,...) lets 20 nodes run at once.
#   stack_resize W H             change the node's screen size (only if STACK_CAN_RESIZE=1)
#   stack_capture FILE.png       the node's own capture of its screen (the reference for SSIM and PSNR)
#   stack_versions               print the versions used
STACK_DESC="headless sway 1.9 + wayvnc v0.10.2 + neatvnc v1.0.3, pixman renderer"
STACK_PROTO=vnc
STACK_CAN_RESIZE=1

stack_start() {
  local i=$1 w=$2 h=$3; shift 3
  NODE_RT=$BENCH_RUN/node$i; clean_dir "$NODE_RT"; mkdir -p -m700 "$NODE_RT"
  NODE_PORT=$((5900 + i))
  cat > "$NODE_RT/sway.conf" <<CONF
default_border none
focus_follows_mouse no
output HEADLESS-1 resolution ${w}x${h}
for_window [app_id=".*"] fullscreen enable
CONF
  start_bg node$i-sway env XDG_RUNTIME_DIR="$NODE_RT" sway -c "$NODE_RT/sway.conf"
  NODE_COMP_PID=$LAST_PID
  NODE_WL=$(wait_socket "$NODE_RT") || die "node sway did not start: $(tail -3 "$RD/logs/node$i-sway.log")"
  wait_for 10 sh -c "ls $NODE_RT/sway-ipc.*.sock"
  NODE_SWAYSOCK=$(ls "$NODE_RT"/sway-ipc.*.sock | head -1)
  [ $# -gt 0 ] && node_run "$@"
  start_bg node$i-wayvnc env XDG_RUNTIME_DIR="$NODE_RT" WAYLAND_DISPLAY="$NODE_WL" wayvnc -n "node$i" -o HEADLESS-1 127.0.0.1 "$NODE_PORT"
  NODE_SRV_PID=$LAST_PID
  wait_for 10 python3 -c "import socket;socket.create_connection(('127.0.0.1',$NODE_PORT)).close()" || die "wayvnc did not listen: $(tail -3 "$RD/logs/node$i-wayvnc.log")"
}
stack_resize() { env XDG_RUNTIME_DIR="$NODE_RT" SWAYSOCK="$NODE_SWAYSOCK" swaymsg output HEADLESS-1 resolution "${1}x${2}" >/dev/null; }
stack_capture() { node_screenshot_wlr "$1"; }
stack_versions() { echo "sway $(sway --version | head -1); $(wayvnc --version 2>&1 | head -2 | tr '\n' ' ')"; }
