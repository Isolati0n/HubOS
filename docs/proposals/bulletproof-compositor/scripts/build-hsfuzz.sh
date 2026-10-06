#!/bin/bash
# builds examples/hsfuzz.rs from the PATCHED tree twice-checked: release + overflow-checks + debug-assertions (the UBSan-equivalent for Rust)
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
mkdir -p $W/dwp/examples
cp $W/hsfuzz.rs $W/dwp/examples/hsfuzz.rs
cd $W/dwp
export CARGO_TARGET_DIR=$W/t-hsf
export CARGO_PROFILE_RELEASE_OVERFLOW_CHECKS=true CARGO_PROFILE_RELEASE_DEBUG_ASSERTIONS=true CARGO_PROFILE_RELEASE_LTO=off CARGO_PROFILE_RELEASE_OPT_LEVEL=1
SECONDS=0
nice -n 15 cargo build --release -j2 --example hsfuzz 2>&1 | grep -E "^error|warning: unused|Finished|could not" -A6 | head -60
echo "HSFUZZ BUILD after ${SECONDS}s"
cp $W/t-hsf/release/examples/hsfuzz $W/bin/hsfuzz-patched 2>&1
ls -la $W/bin/hsfuzz-patched
