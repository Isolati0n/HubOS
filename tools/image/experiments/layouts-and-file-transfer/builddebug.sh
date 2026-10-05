#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# debug build of driftwm (has debug assertions; the release build does not)
W=$LFT
. $W/env.sh
cd $W/src && cargo build 2>&1 | tail -2
