#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
. $LFT/env.sh
cd $W/src
CARGO_TARGET_DIR=$W/target-rel nice -n 10 cargo build --release -j 3 > $W/buildrel.log 2>&1
tail -3 $W/buildrel.log
ls -la $W/target-rel/release/driftwm
