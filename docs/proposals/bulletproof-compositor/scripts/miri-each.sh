#!/bin/bash
# runs every test whose name matches the prefix filters in its own Miri process, so one unsupported foreign call does not stop the rest
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
LABEL=$1; shift
cd $W/dw-pristine
export CARGO_TARGET_DIR=$W/t-miri
export MIRIFLAGS="-Zmiri-disable-isolation"
export MIRI_LIB_SRC=$RUSTUP_HOME/toolchains/nightly-x86_64-unknown-linux-gnu/lib/rustlib/src/rust/library
OUT=$W/miri-$LABEL.tsv; : > $OUT
TESTS=$(nice -n 15 cargo miri test --lib -- --list "$@" 2>/dev/null | grep ": test$" | sed 's/: test$//')
n=0; ok=0; unsup=0; fail=0
for t in $TESTS; do
  n=$((n+1))
  r=$(nice -n 15 timeout 900 cargo miri test --lib -- --exact "$t" 2>&1)
  if echo "$r" | grep -q "test result: ok. 1 passed"; then ok=$((ok+1)); echo -e "ok\t$t" >> $OUT
  elif echo "$r" | grep -q "unsupported operation"; then unsup=$((unsup+1)); echo -e "unsupported\t$t\t$(echo "$r" | grep -m1 'unsupported operation' | cut -c1-120)" >> $OUT
  else fail=$((fail+1)); echo -e "FAIL\t$t\t$(echo "$r" | grep -m1 -E 'error|Undefined|panicked' | cut -c1-160)" >> $OUT; fi
done
echo "$LABEL: total=$n ok=$ok unsupported=$unsup fail=$fail" | tee -a $OUT
