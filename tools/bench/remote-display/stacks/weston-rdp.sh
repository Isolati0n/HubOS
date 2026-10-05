# STACK: Weston 13.0.0 RDP backend (kiosk shell, pixman renderer). The server and the compositor are one program.
# Contract: see wayvnc-sway.sh.
STACK_DESC="Weston 13.0.0 rdp backend, kiosk shell, pixman renderer (RDP over TLS, user name and password not checked)"
STACK_PROTO=rdp
STACK_CAN_RESIZE=0
STACK_APP_AFTER_CLIENT=1   # the node has no seat until a client connects; run.sh starts the scene program after the viewer is connected     # the RDP client announces the size; the node has no command to change it (UNKNOWN whether the display-control channel works, see report)
WESTON_CERT=$BENCH_TMP/certs
# Weston looks for its modules in the fixed path /usr/lib/x86_64-linux-gnu/libweston-13; WESTON_MODULE_MAP points it at the unpacked copies
weston_module_map() { local f; for f in "$BENCH_ROOT"/usr/lib/x86_64-linux-gnu/libweston-13/*.so "$BENCH_ROOT"/usr/lib/x86_64-linux-gnu/weston/*.so; do printf '%s=%s;' "$(basename "$f")" "$f"; done; }

stack_start() {
  local i=$1 w=$2 h=$3; shift 3
  NODE_RT=$BENCH_RUN/node$i; clean_dir "$NODE_RT"; mkdir -p -m700 "$NODE_RT"
  NODE_PORT=$((3390 + i))
  start_bg node$i-weston env XDG_RUNTIME_DIR="$NODE_RT" WESTON_MODULE_MAP="$(weston_module_map)" weston --backend=rdp --renderer=pixman --shell=kiosk --no-config \
    --width="$w" --height="$h" --port="$NODE_PORT" --address=127.0.0.1 --rdp-tls-cert="$WESTON_CERT/node.crt" --rdp-tls-key="$WESTON_CERT/node.key" \
    --socket=wl-weston --debug
  NODE_COMP_PID=$LAST_PID; NODE_SRV_PID=$LAST_PID
  NODE_WL=$(wait_socket "$NODE_RT") || die "weston did not start: $(tail -3 "$RD/logs/node$i-weston.log")"
  [ $# -gt 0 ] && node_run "$@"
  wait_for 10 python3 -c "import socket;socket.create_connection(('127.0.0.1',$NODE_PORT)).close()" || die "weston rdp did not listen"
}
# Weston's capture only completes while a client is connected (TESTED: with no client the request never finishes)
stack_capture() { weston_capture "$1"; }
stack_versions() { weston --version; }
