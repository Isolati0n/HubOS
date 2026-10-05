# CLIENT: TigerVNC viewer 1.13.1 (Ubuntu 24.04 package tigervnc-viewer), an X11 program: runs through the hub's Xwayland.
CLIENT_DESC="TigerVNC viewer 1.13.1 (xtigervncviewer, X11, through Xwayland), full screen"
CLIENT_PROTO=vnc
client_version() { xtigervncviewer -h 2>&1 | grep -m1 -i 'version' ; }
client_start() {
  # Xwayland calls /usr/bin/xkbcomp by a fixed path: run.sh runs everything in a private mount overlay that shows it (lib/ovl.sh)
  start_bg client hub_env env ${STACK_VNC_USER:+VNC_USERNAME=$STACK_VNC_USER VNC_PASSWORD=$STACK_VNC_PASS} xtigervncviewer -FullScreen -SecurityTypes None,X509Plain,X509None,TLSPlain,TLSNone -AutoSelect=0 ${STACK_TLS_CERT:+-X509CA "$STACK_TLS_CERT"} "$1::$2"
  CLIENT_PID=$LAST_PID
}
