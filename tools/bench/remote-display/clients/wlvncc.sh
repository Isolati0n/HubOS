# CLIENT: wlvncc (native Wayland VNC viewer by the wayvnc author), software rendering. One file = one client; run.sh sources it.
# Contract: CLIENT_DESC, CLIENT_PROTO (vnc|rdp), client_version, client_start HOST PORT W H (sets CLIENT_PID; the window must
#   end up fullscreen on the hub, which the hub's sway config does for every window), optional CLIENT_BLOCKED=reason.
CLIENT_DESC="wlvncc (any1/wlvncc @ cc0abf8, 2026-04-29), Wayland-native VNC viewer, -s software renderer"
CLIENT_PROTO=vnc
client_version() { echo "wlvncc cc0abf87c37920540f2439a556e6a480c28f8f (tag-less master, built from source); $(wlvncc --help 2>&1 | head -1)"; }
client_start() {
  start_bg client hub_env wlvncc -s -d -a bench-view ${STACK_TLS_CERT:+-t "$STACK_TLS_CERT"} ${STACK_VNC_USER:+-A "printf '%s\\n%s\\n' $STACK_VNC_USER $STACK_VNC_PASS"} "$1" "$2"
  CLIENT_PID=$LAST_PID
}
