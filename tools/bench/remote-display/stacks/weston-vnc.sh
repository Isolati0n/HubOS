# STACK: Weston 13.0.0 VNC backend (kiosk shell, pixman renderer; needs a TLS certificate). Contract: see wayvnc-sway.sh.
STACK_DESC="Weston 13.0.0 vnc backend, kiosk shell, pixman renderer (VNC over TLS)"
STACK_PROTO=vnc
STACK_CAN_RESIZE=0
STACK_APP_AFTER_CLIENT=1   # the node has no seat until a client connects; run.sh starts the scene program after the viewer is connected
WESTON_CERT=$BENCH_TMP/certs
STACK_VNC_USER=$(id -un); STACK_VNC_PASS=bench   # Weston accepts only the user it runs as (vnc.c: pw_uid != getuid()) and checks the password with PAM; the test PAM service (lib/ovl.sh) accepts any password. Test values, not secrets.
STACK_TLS_CERT=$WESTON_CERT/node.crt   # clients that must be told which certificate to trust read this (self-signed test certificate, no secret)
# Weston looks for its modules in the fixed path /usr/lib/x86_64-linux-gnu/libweston-13; WESTON_MODULE_MAP points it at the unpacked copies
weston_module_map() { local f; for f in "$BENCH_ROOT"/usr/lib/x86_64-linux-gnu/libweston-13/*.so "$BENCH_ROOT"/usr/lib/x86_64-linux-gnu/weston/*.so; do printf '%s=%s;' "$(basename "$f")" "$f"; done; }

stack_start() {
  local i=$1 w=$2 h=$3; shift 3
  NODE_RT=$BENCH_RUN/node$i; clean_dir "$NODE_RT"; mkdir -p -m700 "$NODE_RT"
  NODE_PORT=$((5900 + i))
  start_bg node$i-weston env XDG_RUNTIME_DIR="$NODE_RT" WESTON_MODULE_MAP="$(weston_module_map)" weston --backend=vnc --renderer=pixman --shell=kiosk --no-config \
    --width="$w" --height="$h" --port="$NODE_PORT" --address=127.0.0.1 --vnc-tls-cert="$WESTON_CERT/node.crt" --vnc-tls-key="$WESTON_CERT/node.key" \
    --socket=wl-weston --debug
  NODE_COMP_PID=$LAST_PID; NODE_SRV_PID=$LAST_PID
  NODE_WL=$(wait_socket "$NODE_RT") || die "weston did not start: $(tail -3 "$RD/logs/node$i-weston.log")"
  [ $# -gt 0 ] && node_run "$@"
  wait_for 10 python3 -c "import socket;socket.create_connection(('127.0.0.1',$NODE_PORT)).close()" || die "weston vnc did not listen"
}
# Weston's capture only completes while a client is connected (TESTED: with no client the request never finishes)
stack_capture() { weston_capture "$1"; }
stack_versions() { weston --version; }
