# CLIENT: remote-viewer (virt-viewer 11.0, gtk-vnc 1.3.1), GTK3 on Wayland, VNC.
CLIENT_DESC="remote-viewer 11.0 (virt-viewer, gtk-vnc 1.3.1), GTK on Wayland, full screen"
CLIENT_PROTO=vnc
client_version() { remote-viewer --version; }
client_start() {
  # the node's test certificate is trusted through /etc/pki/CA/cacert.pem, which lib/ovl.sh shows in the private mount overlay
  local hm=$BENCH_TMP/home; mkdir -p "$hm"
  # remote-viewer takes no password on the command line; for a server that wants a login it reads a connection file (.vv, mode 0600)
  local target="vnc://$1:$2"
  if [ -n "${STACK_VNC_USER:-}" ]; then
    target=$RD/rv.vv; ( umask 077; printf '[virt-viewer]\ntype=vnc\nhost=%s\nport=%s\nusername=%s\npassword=%s\n' "$1" "$2" "$STACK_VNC_USER" "$STACK_VNC_PASS" > "$target" )
  fi
  start_bg client hub_env env HOME="$hm" GDK_BACKEND=wayland remote-viewer --full-screen "$target"
  CLIENT_PID=$LAST_PID
}
