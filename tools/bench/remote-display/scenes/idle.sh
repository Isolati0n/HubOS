# SCENE: idle desktop. A white terminal window shows one page of text; nothing changes. Contract for scene files:
#   SCENE_DESC, SCENE_NEEDS ("" or "resize"), scene_setup W H (sets SCENE_APP array and PROBE_X/PROBE_Y/PROBE_EXPECT/PROBE_TOL), scene_run
SCENE_DESC="idle desktop: a terminal shows one page of text, nothing changes (15 s)"
SCENE_DUR=15
scene_setup() {
  local w=$1 h=$2
  foot_cmd sh -c "head -n 120 $BENCH_TMP/media/corpus.txt; exec sleep 100000"
  SCENE_APP=("${FOOT[@]}")
  PROBE_X=$((w - 12)); PROBE_Y=$((h / 2)); PROBE_EXPECT=205; PROBE_TOL=10
}
scene_run() { sleep 5; measure_window m "$SCENE_DUR"; }
