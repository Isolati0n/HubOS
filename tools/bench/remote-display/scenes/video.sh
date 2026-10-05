SCENE_DESC="video playback: mpv plays a 1280x720, 30 fps moving test pattern (H.264 file, software decoding on the node) scaled to the screen; a frame-number bar code is read on the hub to count dropped frames"
SCENE_DUR=12
scene_setup() {
  local w=$1 h=$2
  SCENE_APP=(mpv --no-config --vo=wlshm --no-audio --fs --loop-file=inf --no-osc --no-input-default-bindings --really-quiet "$BENCH_TMP/media/video.mp4")
  PROBE_X=
  VIDEO_SCALE=$(python3 -c "print($w/1280)")
}
scene_run() {
  sleep 4
  measure_window m "$SCENE_DUR"
  # phase 2: read the bar code on the hub for 10 s (pixelwatch costs a little hub CPU, so it runs after the CPU window)
  local pts=() k x y
  y=$(python3 -c "print(int(20*$VIDEO_SCALE))")
  for k in $(seq 0 15); do x=$(python3 -c "print(int((40+80*$k)*$VIDEO_SCALE))"); pts+=("$x,$y"); done
  start_bg bars hub_env pixelwatch -t 10 "${pts[@]}"; local bp=$LAST_PID
  local t0; t0=$(now_ns); sleep 10.5; local t1; t1=$(now_ns)
  python3 "$BENCH_DIR/metrics/analyze.py" bars "$RD/logs/bars.log" "$t0" "$t1" | res_kv
  kill $bp 2>/dev/null
}
