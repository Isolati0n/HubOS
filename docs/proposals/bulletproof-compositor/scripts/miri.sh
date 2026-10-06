#!/bin/bash
# usage: miri.sh LABEL TEST_FILTER...   runs cargo miri test --lib on driftwm (pristine tree) with the given test name filters
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
LABEL=$1; shift
cd $W/dw-pristine
export CARGO_TARGET_DIR=$W/t-miri
export MIRIFLAGS="${MIRIFLAGS:--Zmiri-disable-isolation}"
export MIRI_LIB_SRC=$RUSTUP_HOME/toolchains/nightly-x86_64-unknown-linux-gnu/lib/rustlib/src/rust/library
SECONDS=0
nice -n 15 timeout ${MIRI_TIMEOUT:-3000} cargo miri test --lib -j2 -- "$@" > $W/miri-$LABEL.log 2>&1
echo "exit=$? after ${SECONDS}s" >> $W/miri-$LABEL.log
tail -25 $W/miri-$LABEL.log | cut -c1-240
