# CLIENT: FreeRDP 3.32.0 X11 client (xfreerdp3), through the hub's Xwayland.
CLIENT_DESC="FreeRDP 3.32.0 xfreerdp3 (X11, through Xwayland), full screen, TLS security"
CLIENT_PROTO=rdp
client_version() { xfreerdp3 --version 2>&1 | head -1; }
client_start() {
  start_bg client hub_env xfreerdp3 "/v:$1:$2" /u:bench /p:bench /cert:ignore /sec:tls /f "/size:${3}x${4}" +clipboard /title:bench-rdp
  CLIENT_PID=$LAST_PID
}
