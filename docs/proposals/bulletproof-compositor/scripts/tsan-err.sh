#!/bin/bash
B=${BC_WORK:?set BC_WORK to your work folder}
. $B/env.sh
cd $B/dwp-tsan
export CARGO_TARGET_DIR=$B/t-tsan
RUSTFLAGS="-Zsanitizer=thread" nice -n 15 cargo build -j2 -Zbuild-std --target x86_64-unknown-linux-gnu 2>&1 | grep -E "^error" -A14 | head -40 | cut -c1-220
