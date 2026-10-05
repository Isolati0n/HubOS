# CLIENT: FreeRDP 3.32.0 Wayland client (wlfreerdp3). Deprecated by its authors, but it is the only FreeRDP client that is a Wayland window.
CLIENT_DESC="FreeRDP 3.32.0 wlfreerdp3 (Wayland), full screen, TLS security"
CLIENT_PROTO=rdp
client_version() { wlfreerdp3 --version 2>&1 | head -1; }
client_start() {
  start_bg client hub_env wlfreerdp3 "/v:$1:$2" /u:bench /p:bench /cert:ignore /sec:tls /f "/size:${3}x${4}" +clipboard /title:bench-rdp /wm-class:bench-rdp
  CLIENT_PID=$LAST_PID
}
