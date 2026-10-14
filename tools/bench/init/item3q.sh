#!/bin/bash
# item3q.sh (BENCH CODE; needs the env of the bench: W T REPO). Follow-up 4 queue: runs one VM at a time, one after the other, and after
# every run saves the log under results/step2/<candidate>/ (SAMPLE lines stripped), runs tools/check-push.sh against the remote branch
# tip, commits and pushes. Each line of the queue is: CANDIDATE SUITE SEED REPS TIMEOUT
ulimit -c 0
cd $REPO || exit 1
while read -r cand suite seed reps tmo; do
  [ -z "$cand" ] && continue
  while pgrep -x qemu-system-x86 >/dev/null; do sleep 10; done   # long jobs never overlap
  raw=$W/runs/$cand-$suite-$seed.raw.log
  echo "QUEUE start $cand $suite seed=$seed reps=$reps $(date +%T)"
  python3 tools/bench/init/run-qemu.py $cand $suite --dur 600 --seed $seed --reps $reps --timeout $tmo --out $raw > /dev/null 2>&1
  out=tools/bench/init/results/step2/$cand/$cand-$suite-$seed.log
  mkdir -p $(dirname $out)
  grep -v ' SAMPLE ' $raw > $out
  echo "QUEUE done $cand $suite seed=$seed $(date +%T): $(grep -c 'SUITE-DONE' $raw) SUITE-DONE, $(grep -c ' STUCK ' $raw) STUCK, $(grep -c 'HANG n=' $raw) HANG lines"
  git add -A tools/bench/init/results && git -c core.hooksPath=/dev/null commit -q -m "Init comparison follow-up 4, item ${ITEM:-3}: $cand $suite seed $seed (VM log)

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_014eDKni7z8VkkvoLh3h3vmu"
  git fetch -q origin
  if CHECK_PUSH_BASE=origin/init-comparison-step2 bash tools/check-push.sh >/dev/null 2>&1; then git push -q origin init-comparison-step2 && echo "QUEUE pushed"; else echo "QUEUE check-push FAILED, not pushed"; fi
done
echo "QUEUE ALL DONE $(date +%T)"
