#!/bin/bash
# re-run the two Miri tests that failed in batch b2 with a 50-minute limit and record how they ended (exit status, time)
ulimit -c 0
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
cd $W/dw-pristine
export CARGO_TARGET_DIR=$W/t-miri
export MIRIFLAGS="-Zmiri-disable-isolation"
export MIRI_LIB_SRC=$RUSTUP_HOME/toolchains/nightly-x86_64-unknown-linux-gnu/lib/rustlib/src/rust/library
OUT=$W/miri-fail.txt; : > $OUT
for t in stage::tests::harness::fit_fullscreen_round_trips_restore_saved_sizes stage::tests::harness::random_op_sequences_preserve_invariants; do
  s=$(date +%s)
  nice -n 15 timeout 3000 cargo miri test --lib -- --exact "$t" > $W/miri-fail-one.log 2>&1
  rc=$?
  echo "$t exit=$rc after $(( $(date +%s) - s )) s: $(tail -3 $W/miri-fail-one.log | tr '\n' ' ' | cut -c1-300)" >> $OUT
done
echo done >> $OUT
