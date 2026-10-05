#!/bin/bash
# Runs the level-of-detail experiments of docs/proposals/remote-display-lod.md, one fresh fake node per experiment.
#   BENCH_TMP=/tmp/bench ./run-lod.sh [EXPERIMENT...]      (no argument = all)
# Output: results/NAME.txt (the exact command on the first line, then rfbprobe's table).
# Needs the unpacked programs and builds that tools/bench/remote-display/setup.sh makes (branch bench-remote-display-clean).
# Loopback only, 4 shared virtual cores, software rendering: the numbers say which request style costs more
# than which, not how fast any real machine is.
set -u
ulimit -c 0
cd "$(dirname "$0")"
export BENCH_TMP=${BENCH_TMP:-/tmp/bench}
OUT=results; mkdir -p $OUT
PORT=5901

# one NAME SCENE WxH "WAYVNC ARGS" PROBE_ARGS...
one() {
  local name=$1 scene=$2 size=$3 wargs=$4; shift 4
  local w=${size%x*} h=${size#*x}
  ./node.sh start "$scene" "$w" "$h" $PORT $wargs > $OUT/.pids.$$ || { echo "node start failed" >&2; ./node.sh stop; return 1; }
  sleep 3
  local pids=""
  while read -r n p; do pids="$pids --pid $n=$p"; done < $OUT/.pids.$$
  { echo "scene=$scene size=$size wayvnc_args='$wargs'"; echo "cmd: rfbprobe.py --port $PORT $* $pids"; python3 rfbprobe.py --port $PORT "$@" $pids; } > $OUT/$name.txt 2>&1
  { echo "# server log lines that mention an assertion or abort:"; grep -a -i 'assert\|abort' $BENCH_TMP/lod/wayvnc.log | cut -c1-200; } >> $OUT/$name.txt
  ./node.sh stop
  rm -f $OUT/.pids.$$
  cat $OUT/$name.txt
}

want() { [ $# -eq 0 ] && return 0; for x in "${WANT[@]}"; do [ "$x" = "$1" ] && return 0; done; return 1; }
WANT=("$@"); [ ${#WANT[@]} -eq 0 ] && WANT=(pacing cu resize fence enc maxfps resume)

want pacing && for s in clock scroll idle; do
  one pacing-$s $s 1280x720 "" --phases 'normal:6,paced@0.1:6,paced@1:6,none:6,normal:6'
done
want cu && for s in clock scroll; do
  one cu-$s $s 1280x720 "" --phases 'normal:5,cu:6,curegion@600+400+100+100:6,curegion@0+0+300+100:6,cuoff:6,normal:5'
done
want resize && for s in scroll clock; do
  one resize-$s $s 1280x720 "" --phases 'normal:5,resize@640x360:6,resize@320x180:6,resize@1280x720:6,normal:3'
done
want fence && {
  one fence-after-normal-scroll scroll 1280x720 "" --phases 'normal:3,fence:5'
  one fence-idle idle 1280x720 "" --phases 'normal:3,fence:5'
  one fence-after-cu-idle idle 1280x720 "" --phases 'cu:3,fence:5'
  one fence-after-cuoff-scroll scroll 1280x720 "" --phases 'cu:2,cuoff:1,fence:4'
  one fence-during-cu-scroll scroll 1280x720 "" --phases 'cu:2,fence:4'
}
want enc && one enc-raw-scroll scroll 1280x720 "" --enc raw --phases 'normal:6,paced@0.1:6,none:3'
want maxfps && for f in 30 10 5; do
  one maxfps-$f-scroll scroll 1280x720 "-f $f" --phases 'normal:6,none:6'
done
want resume && for s in clock scroll; do
  one resume-$s $s 1280x720 "" --phases 'normal:3,none:10,normal:3,none:3,paced@1:3,normal:3'
done
