#!/bin/bash
# final fuzz round on the candidate p25: Wayland fuzzer (release), Wayland fuzzer (ASan, overflow checks), IPC fuzzer (release, 3 seeds), pure-logic fuzzer
B=${BC_WORK:?set BC_WORK to your work folder}
export FZDISPLAY=:93
bash $B/fzloop.sh $B/bin/driftwm-p25 rel25 1101 1116 120
ASAN_OPTIONS=detect_leaks=1:symbolize=1 bash $B/fzloop.sh $B/bin/driftwm-asan-p25 asan25 1201 1208 100
for s in 11 12 13; do
  rm -rf $B/ipcf$s
  bash $B/py.sh $B/ipcfuzz.py 240 $B/ipcf$s $s $B/bin/driftwm-p25 6000000 > $B/ipcf$s.out 2>&1
done
rm -rf $B/hsf-final; mkdir -p $B/hsf-final
for t in toml parse json geom; do
  nice -n 15 $B/bin/hsfuzz-patched $t 300 7 $B/hsf-final/$t > $B/hsf-final/$t-summary.txt 2>&1
done
echo FINAL-FUZZ-DONE > $B/final-fuzz.done
