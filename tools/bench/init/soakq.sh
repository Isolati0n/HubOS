#!/bin/bash
# soakq.sh CHUNK_SECONDS "CAND1 CAND2 ..." CHUNKS [FIRST_SEED]  (BENCH CODE; needs the env of the bench: W T REPO)
# One candidate at a time, CHUNKS chunks each (new seed per chunk). Stops a candidate early when a chunk shows 2 STUCK states
# (a state only a reboot clears). After every chunk: strips the SAMPLE spam, saves the log and the memory analysis under
# tools/bench/init/results/step2/soak/, runs tools/check-push.sh, commits and pushes, prints "chunk N pushed, TIME".
ulimit -c 0
len=$1; cands=$2; chunks=$3; seed=${4:-6000}
R=$REPO/tools/bench/init/results/step2/soak; mkdir -p $R
n=0
for c in $cands; do
  for k in $(seq 1 $chunks); do
    n=$((n+1)); seed=$((seed+10))
    raw=$W/runs/soak-$c-${PREFIX:-chunk}$k.raw.log
    python3 $REPO/tools/bench/init/run-qemu.py $c soak --dur $len --seed $seed --timeout $((len+900)) --out $raw > /dev/null 2>&1 &
    pid=$!
    stopped=""
    while kill -0 $pid 2>/dev/null; do
      sleep 20
      if [ "$(grep -c ' STUCK ' $raw 2>/dev/null)" -ge 2 ]; then
        stopped="early stop: 2 STUCK states"
        kill $pid 2>/dev/null; sleep 2
        for q in $(ps -eo pid,comm | awk '$2=="qemu-system-x86"{print $1}'); do kill $q; done
        break
      fi
    done
    wait $pid 2>/dev/null
    out=$R/$c-${PREFIX:-chunk}$k.log
    { echo "H# chunk $k of candidate $c, seed $seed, planned $len s ${stopped:+- $stopped}"; grep -v ' SAMPLE ' $raw; } > $out
    python3 -I $REPO/tools/bench/init/soak_analyze.py $out > $R/$c-${PREFIX:-chunk}$k.analysis.txt 2>&1
    echo "STUCK states in chunk: $(grep -c ' STUCK ' $out); SOAK-END lines: $(grep -c 'SOAK-END' $out); launches: $(grep -c '^H# launch' $out); self-reboots (reboot: Restarting system): $(grep -c 'reboot: Restarting system' $out); events that needed a person (ok=0): $(grep ' EV ev=' $out | grep -c 'ok=0'); UNRECOVERED lines: $(grep -c ' UNRECOVERED ' $out)" >> $R/$c-${PREFIX:-chunk}$k.analysis.txt
    cd $REPO && git add -A tools/bench/init/results/step2/soak && git commit -q -m "Step 2 soak: $c ${PREFIX:-chunk} $k ${stopped}

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_014eDKni7z8VkkvoLh3h3vmu" && git fetch -q origin && CHECK_PUSH_BASE=origin/init-comparison-step2 bash tools/check-push.sh && git push -q origin init-comparison-step2 && echo "chunk $n pushed, $(date -u +%T)" >> $W/runs/soakq.progress
    [ -n "$stopped" ] && break
  done
done
echo done > $W/runs/soakq.done
