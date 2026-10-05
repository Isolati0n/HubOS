SCENE_DESC="terminal output flood: a terminal prints text as fast as it can, like a build log (15 s)"
SCENE_DUR=15
scene_setup() {
  local w=$1 h=$2
  foot_cmd python3 "$BENCH_DIR/scenes/emit.py" "$BENCH_TMP/media/corpus.txt" 0
  SCENE_APP=("${FOOT[@]}")
  PROBE_X=$((w - 12)); PROBE_Y=$((h / 2)); PROBE_EXPECT=205; PROBE_TOL=10
}
scene_run() { sleep 5; measure_window m "$SCENE_DUR"; }
