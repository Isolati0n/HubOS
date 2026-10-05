SCENE_DESC="reconnect: (1) the viewer is closed and started again: time until the node's picture is back; (2) the network link is cut (the byte counter closes the connection): does the viewer come back by itself"
SCENE_DUR=0
SCENE_KEEP_PROBE=1
. "$(dirname "${BASH_SOURCE[0]}")/idle.sh"
scene_run() {
  sleep 4
  # (1) new viewer
  local t0 ms
  kill_tree "$CLIENT_PID"; sleep 1.5
  t0=$(now_ns); client_start 127.0.0.1 "$METER_LISTEN" "$W" "$H"
  ms=$(wait_pixel "$t0" "$PROBE_EXPECT" "$PROBE_TOL" 40); res reconnect_new_viewer_ms "$ms"
  # (2) link cut. Is the same viewer process still alive, did it open a new connection, did the picture return?
  sleep 3
  local acc0; acc0=$(meter_read | cut -d' ' -f3)
  local alive0=0; kill -0 "$CLIENT_PID" 2>/dev/null && alive0=1
  t0=$(now_ns); kill -USR1 "$METER_PID"
  sleep 15
  local acc1; acc1=$(meter_read | cut -d' ' -f3)
  res link_cut_viewer_alive_after_15s "$(kill -0 "$CLIENT_PID" 2>/dev/null && echo 1 || echo 0)"
  res link_cut_new_connections_in_15s "$(( acc1 - acc0 ))"
  # did the node picture come back on the hub after the cut? look for a bright probe pixel after the cut + 1 s
  ms=$(wait_pixel "$(( t0 + 1000000000 ))" "$PROBE_EXPECT" "$PROBE_TOL" 3); res link_cut_picture_back_ms "$ms"
}
