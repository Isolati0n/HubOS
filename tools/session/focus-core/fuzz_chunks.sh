#!/bin/bash
# fuzz_chunks.sh LANG BINARY CHUNKS CASES_PER_CHUNK FIRST_SEED : runs CHUNKS fuzz runs, each with a new seed, each under a time limit
# (a run that does not finish in 20 minutes counts as a HANG). Appends one line per chunk to results/LANG-fuzz.txt.
ulimit -c 0
lang=$1; bin=$2; chunks=$3; cases=$4; seed=$5
out="$(dirname "$0")/results/$lang-fuzz.txt"
for i in $(seq 1 $chunks); do
  s=$((seed + i - 1))
  line=$(timeout 1200 $bin $s $cases 2>&1); rc=$?
  case $rc in
    0) echo "$line" >> "$out" ;;
    124) echo "$lang fuzz: seed=$s cases=$cases HANG (killed after 1200 s)" >> "$out" ;;
    *) echo "$lang fuzz: seed=$s cases=$cases FAILED exit=$rc: $line" >> "$out" ;;
  esac
done
