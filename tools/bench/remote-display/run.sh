#!/bin/bash
# One measurement: run.sh STACK CLIENT SCENE RES [RUN_NUMBER]
#   STACK  = a file in stacks/ (without .sh)      CLIENT = a file in clients/      SCENE = a file in scenes/
#   RES    = 720p | 1080p | 4k                    RUN_NUMBER defaults to 1
# Writes results/raw/STACK__CLIENT__SCENE__RES__rN.txt (key=value lines). An existing file is not run again (FORCE=1 overrides).
# Needs: ./setup.sh all   and   ./metrics/build-tools.sh   and   ./scenes/gen-media.sh   (once).
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
if [ -z "${BENCH_OVL:-}" ]; then export BENCH_OVL=1; exec "$HERE/lib/ovl.sh" "$HERE/run.sh" "$@"; fi   # private /usr/bin overlay, see lib/ovl.sh
[ $# -ge 4 ] || { sed -n '2,6p' "$0"; exit 2; }
STACK=$1 CLIENT=$2 SCENE=$3 RESNAME=$4 RUN=${5:-1}
. "$HERE/env.sh"; . "$HERE/lib/common.sh"
case $RESNAME in 720p) W=1280 H=720;; 1080p) W=1920 H=1080;; 4k) W=3840 H=2160;; *) die "RES must be 720p, 1080p or 4k";; esac
OUTDIR=${RESULTS:-$HERE/results/raw}; mkdir -p "$OUTDIR"
FINAL=$OUTDIR/${STACK}__${CLIENT}__${SCENE}__${RESNAME}__r${RUN}.txt
[ -e "$FINAL" ] && [ -z "${FORCE:-}" ] && { log "skip (exists): $FINAL"; exit 0; }
RES_TMP=$RD/result.txt; : > "$RES_TMP"
for f in stacks/$STACK clients/$CLIENT scenes/$SCENE; do [ -f "$HERE/$f.sh" ] || die "no such file: $f.sh"; done
. "$HERE/stacks/$STACK.sh"; . "$HERE/clients/$CLIENT.sh"; . "$HERE/scenes/$SCENE.sh"
finish() { local st=$1; stop_all; [ -n "${KEEP_LOGS:-}" ] && { mkdir -p "$KEEP_LOGS"; cp -r "$RD/logs" "$KEEP_LOGS/$(basename "$FINAL" .txt)"; }; { echo "status=$st"; cat "$RES_TMP"; } > "$FINAL.tmp"; mv "$FINAL.tmp" "$FINAL"; clean_dir "$RD"; log "done: $FINAL ($st)"; exit 0; }
trap 'finish "fail:interrupted"' INT TERM

res stack "$STACK"; res client "$CLIENT"; res scene "$SCENE"; res res "${W}x${H}"; res run "$RUN"; res date "$(date -u +%FT%TZ)"
res stack_desc "$STACK_DESC"; res client_desc "$CLIENT_DESC"
if [ "$STACK_PROTO" != "$CLIENT_PROTO" ]; then finish "unsupported:stack speaks $STACK_PROTO, client speaks $CLIENT_PROTO"; fi
if [ -n "${STACK_ONLY_RES:-}" ] && [ "$STACK_ONLY_RES" != "$RESNAME" ]; then finish "unsupported:stack only runs at $STACK_ONLY_RES"; fi
if [ "${SCENE_NEEDS:-}" = resize ] && [ "$STACK_CAN_RESIZE" != 1 ]; then finish "unsupported:stack cannot change the node screen size"; fi
if [ -n "${CLIENT_BLOCKED:-}" ]; then finish "unsupported:$CLIENT_BLOCKED"; fi
if [ -n "${CLIENT_BLOCKED_STACK_PROTO_NOTE:-}" ]; then :; fi

NODE_APP_PIDS=(); PROBE_X=; scene_setup "$W" "$H"
res load_start "$(load_now)"; res stack_versions "$(stack_versions | tr '\n' ' ')"; res client_version "$(client_version 2>&1 | head -1)"
hub_start "$W" "$H" || finish "fail:hub"
[ -n "${SCENE_HUB_INPUT:-}" ] && start_bg hubinput hub_env sh -c "tail -f /dev/null | hubclick $W $H"
# Weston's RDP and VNC backends have no seat (no keyboard or mouse) until a client has connected, and foot refuses to start without one
# (TESTED: "no seats available"). Those stacks set STACK_APP_AFTER_CLIENT=1: the scene program starts after the viewer connected, so for them
# "time to first frame" also contains the start of the scene program.
AFTER=${STACK_APP_AFTER_CLIENT:-}; [ -n "${SCENE_APP_TOLERANT:-}" ] && AFTER=   # benchapp copes with a seat that appears later
if [ "$AFTER" = 1 ]; then stack_start 1 "$W" "$H" || finish "fail:node"; else stack_start 1 "$W" "$H" "${SCENE_APP[@]}" || finish "fail:node"; fi
export NODE_RT NODE_WL
METER_LISTEN=$((21000 + NODE_PORT % 1000)); meter_start "$METER_LISTEN" "$NODE_PORT"
[ -n "$PROBE_X" ] && probe_start "$PROBE_X,$PROBE_Y"
T_LAUNCH=$(now_ns)
client_start 127.0.0.1 "$METER_LISTEN" "$W" "$H" || finish "fail:client_start"
if [ "$AFTER" = 1 ]; then
  wait_for 30 sh -c "[ \$(cut -d' ' -f3 $METER_STATS) -ge 1 ]"; sleep 1; node_run "${SCENE_APP[@]}"
fi
if [ -n "$PROBE_X" ]; then
  ms=$(wait_pixel "$T_LAUNCH" "$PROBE_EXPECT" "$PROBE_TOL" 60); res time_to_first_frame_ms "$ms"
  [ "$ms" = none ] && { res client_log "$(tail -c 300 "$RD/logs/client.log" | tr '\n' ' ')"; finish "fail:no first frame in 60 s"; }
  [ -z "${SCENE_KEEP_PROBE:-}" ] && kill "$PROBE_PID" 2>/dev/null   # reconnect and resize keep watching
else
  sleep 8
fi
kill -0 "$CLIENT_PID" 2>/dev/null || { res client_log "$(tail -c 300 "$RD/logs/client.log" | tr '\n' ' ')"; finish "fail:viewer exited"; }
scene_run
[ "$STACK_PROTO" = vnc ] && python3 "$HERE/metrics/analyze.py" encodings "$METER_STATS.c2s" | res_kv
res load_end "$(load_now)"
finish ok
