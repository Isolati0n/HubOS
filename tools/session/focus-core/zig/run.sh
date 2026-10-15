#!/bin/sh
# Builds the Zig candidate. Usage: ZIG=/path/to/zig sh run.sh   (output in ./out)
set -e
cd "$(dirname "$0")"
ZIG=${ZIG:-zig}
mkdir -p out
cp ../traces/all.trace src/all.trace
"$ZIG" build-exe -O ReleaseSafe -femit-bin=out/fuzz src/fuzz.zig
"$ZIG" build-exe -O ReleaseSafe -femit-bin=out/trace src/trace.zig
