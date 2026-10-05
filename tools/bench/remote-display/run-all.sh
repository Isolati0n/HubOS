#!/bin/bash
# Runs the whole matrix (or a part of it). Resumable: finished results are skipped by run.sh.
#   STACKS="..." CLIENTS="..." SCENES="..." RESES="..." RUNS=3 ./run-all.sh
# Defaults: every stack, client and scene file, 1080p, 3 runs. Order: run number outermost, so a stopped campaign has N=1 everywhere before N=2.
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
list() { ls "$HERE/$1" | sed 's/\.sh$//' | tr '\n' ' '; }
STACKS=${STACKS:-$(list stacks)}; CLIENTS=${CLIENTS:-$(list clients)}; SCENES=${SCENES:-$(list scenes | tr ' ' '\n' | grep -v -E 'benchapp|emit|gen-media' | tr '\n' ' ')}
RESES=${RESES:-1080p}; RUNS=${RUNS:-3}
proto() { grep -m1 "^$2=" "$HERE/$1.sh" | cut -d= -f2; }
for r in $(seq 1 "$RUNS"); do for res in $RESES; do for sc in $SCENES; do for st in $STACKS; do for cl in $CLIENTS; do
  [ "$(proto stacks/$st STACK_PROTO)" = "$(proto clients/$cl CLIENT_PROTO)" ] || continue
  only=$(proto stacks/$st STACK_ONLY_RES | sed 's/ *#.*//'); [ -n "$only" ] && [ "$only" != "$res" ] && continue
  "$HERE/run.sh" "$st" "$cl" "$sc" "$res" "$r"
done; done; done; done; done
