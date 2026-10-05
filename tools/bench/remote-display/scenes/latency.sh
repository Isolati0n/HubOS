SCENE_DESC="input-to-pixel delay: a click on the hub makes a square on the node turn black/white; pixelwatch on the hub sees the change; 20 clicks, 0.7 s apart. Also time to first frame."
SCENE_APP_TOLERANT=1
SCENE_DUR=0
scene_setup() {
  local w=$1 h=$2
  SCENE_APP=(benchapp key)
  PROBE_X=$((w - 32)); PROBE_Y=$((h - 32)); PROBE_EXPECT=205   # the solid light-blue patch benchapp key draws in its bottom-right corner
  PROBE_TOL=14
}
scene_run() {
  sleep 3
  # two probe points: the square (20,20) and the first one is not needed again; restart pixelwatch on the square
  kill "$PROBE_PID" 2>/dev/null; sleep 0.3
  start_bg probe2 hub_env pixelwatch -t 60 20,20; local pp=$LAST_PID
  mkfifo "$RD/click.fifo"
  start_bg hubclick hub_env sh -c "hubclick $W $H < $RD/click.fifo >> $RD/clicks.txt"
  exec 7> "$RD/click.fifo"
  sleep 1.5
  echo click >&7; sleep 1       # warm-up click (not counted)
  : > "$RD/clicks.txt"
  local i; for i in $(seq 1 20); do echo click >&7; sleep 0.7; done
  sleep 1; exec 7>&-
  kill $pp 2>/dev/null
  python3 "$BENCH_DIR/metrics/analyze.py" input "$RD/logs/probe2.log" "$RD/clicks.txt" "${NODE_APP_LOG:-$RD/logs/app-benchapp.log}" | res_kv
}
