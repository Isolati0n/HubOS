#!/bin/bash
# usage: build1.sh SRCDIR TARGETDIR OUTNAME [extra cargo args]
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
S=$1; T=$2; N=$3; shift 3
mkdir -p $W/bin
cd $S
export CARGO_TARGET_DIR=$T
SECONDS=0
nice -n 15 cargo build --release -j2 "$@" 2>&1 | tail -15
echo "BUILD $N after ${SECONDS}s"
cp $T/release/driftwm $W/bin/$N && ls -la $W/bin/$N
