#!/bin/bash
# Scale test: N simultaneous sessions (default 20) from N fake nodes into ONE hub. Measures the hub's CPU and memory.
#   ./scale.sh STACK CLIENT [N] [RES] [RUN]        e.g.  ./scale.sh wayvnc-sway wlvncc 20 1080p 1
# Each node is its own headless compositor + server with a static terminal page (so 20 x 1080p screens exist in RAM).
# The hub is one headless sway with a 4K output; the N windows are tiled (not fullscreen). Phase "idle": nothing changes.
# Phase "one_busy": node 1 prints 30 lines a second, the other N-1 stay idle. Result: results/raw/scale__STACK__CLIENT__N__RES__rRUN.txt
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
if [ -z "${BENCH_OVL:-}" ]; then export BENCH_OVL=1; exec "$HERE/lib/ovl.sh" "$HERE/scale.sh" "$@"; fi
STACK=$1 CLIENT=$2 N=${3:-20} RESNAME=${4:-1080p} RUN=${5:-1}
. "$HERE/env.sh"; . "$HERE/lib/common.sh"
case $RESNAME in 720p) W=1280 H=720;; 1080p) W=1920 H=1080;; 4k) W=3840 H=2160;; esac
OUTDIR=${RESULTS:-$HERE/results/raw}; mkdir -p "$OUTDIR"
FINAL=$OUTDIR/scale__${STACK}__${CLIENT}__${N}__${RESNAME}__r${RUN}.txt
[ -e "$FINAL" ] && [ -z "${FORCE:-}" ] && { log "skip (exists): $FINAL"; exit 0; }
RES_TMP=$RD/result.txt; : > "$RES_TMP"
. "$HERE/stacks/$STACK.sh"
# CLIENT=none starts only the nodes (no hub, no viewers): the baseline that tells how much of the memory belongs to the nodes
NOHUB=; if [ "$CLIENT" = none ]; then NOHUB=1; else . "$HERE/clients/$CLIENT.sh"; fi
finish() { local st=$1; stop_all; { echo "status=$st"; cat "$RES_TMP"; } > "$FINAL.tmp"; mv "$FINAL.tmp" "$FINAL"; clean_dir "$RD"; log "done: $FINAL ($st)"; exit 0; }
trap 'finish "fail:interrupted"' INT TERM
res stack "$STACK"; res client "$CLIENT"; res sessions "$N"; res node_res "${W}x${H}"; res run "$RUN"; res date "$(date -u +%FT%TZ)"; res load_start "$(load_now)"
res free_mem_before_mb "$(awk '/MemAvailable/ {print int($2/1024)}' /proc/meminfo)"
foot_cmd sh -c "head -n 120 $BENCH_TMP/media/corpus.txt; exec sleep 100000"; IDLEAPP=("${FOOT[@]}")
[ -z "$NOHUB" ] && { HUB_FULLSCREEN=0 hub_start 3840 2160 || finish "fail:hub"; }
NODE_PIDS=(); VIEW_PIDS=()
for i in $(seq 1 "$N"); do
  NODE_APP_PIDS=()
  if [ "$i" = 1 ]; then foot_cmd python3 "$BENCH_DIR/scenes/emit.py" "$BENCH_TMP/media/corpus.txt" 0.001; ONE=("${FOOT[@]}"); stack_start $i "$W" "$H" "${ONE[@]}" || finish "fail:node $i"
  else stack_start $i "$W" "$H" "${IDLEAPP[@]}" || finish "fail:node $i"; fi
  NODE_PIDS+=("$NODE_COMP_PID" "$NODE_SRV_PID" "${NODE_APP_PIDS[@]}")
  [ "$i" = 1 ] && N1_RT=$NODE_RT && N1_WL=$NODE_WL
done
res nodes_started "$N"
[ -z "$NOHUB" ] && for i in $(seq 1 "$N"); do
  client_start 127.0.0.1 $((5900 + i)) "$W" "$H" || finish "fail:client $i"; VIEW_PIDS+=("$CLIENT_PID"); sleep 0.7
done
sleep 15
alive=0; for p in "${VIEW_PIDS[@]}"; do kill -0 "$p" 2>/dev/null && alive=$((alive+1)); done; res viewers_alive "$alive"
LEAF=(); for p in "${VIEW_PIDS[@]}"; do LEAF+=("$(leaf_pid "$p")"); done   # the viewer itself, not the shell that start_bg put in front of it
VP=$(IFS=,; echo "${LEAF[*]}"); NP=$(IFS=,; echo "${NODE_PIDS[*]}")
phase() {   # phase NAME SECONDS
  python3 "$BENCH_DIR/metrics/procstat.py" snap "$RD/p1.json" "hub_viewers=$VP" "hub_comp=${HUB_COMP_PID:-0}" "all_nodes=$NP"
  sleep "$2"
  python3 "$BENCH_DIR/metrics/procstat.py" snap "$RD/p2.json" "hub_viewers=$VP" "hub_comp=${HUB_COMP_PID:-0}" "all_nodes=$NP"
  local l c r m n p; res "${1}_load" "$(load_now)"
  while read -r l c r m n p; do res "${1}_${l}_cpu" "$c"; res "${1}_${l}_rss_mb" "$r"; res "${1}_${l}_pss_mb" "$p"; res "${1}_${l}_procs" "$n"; done < <(python3 "$BENCH_DIR/metrics/procstat.py" diff "$RD/p1.json" "$RD/p2.json")
}
res free_mem_with_all_mb "$(awk '/MemAvailable/ {print int($2/1024)}' /proc/meminfo)"
phase idle 30
# one busy: node 1 already runs emit.py at a very low rate (0.001 lines/s = idle); restart it fast by signalling is not possible,
# so start a second terminal printer on node 1 (a new window, fullscreen)
foot_cmd python3 "$BENCH_DIR/scenes/emit.py" "$BENCH_TMP/media/corpus.txt" 30
XDG_RUNTIME_DIR=$N1_RT WAYLAND_DISPLAY=$N1_WL start_bg busy "${FOOT[@]}"
sleep 6
phase one_busy 20
finish ok
